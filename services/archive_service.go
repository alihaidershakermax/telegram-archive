package services

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"telegram-archive-bot/config"
	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

// ── In-memory cache ─────────────────────────────────────────

type cacheEntry[T any] struct {
	data []T
	ts   time.Time
}

var (
	categoriesCache   *cacheEntry[models.Category]
	categoriesCacheMu sync.RWMutex

	subjectsCache   = map[string]*cacheEntry[models.Subject]{}
	subjectsCacheMu sync.RWMutex

	filesCache   = map[string]*cacheEntry[models.File]{}
	filesCacheMu sync.RWMutex
)

func cacheTTL() time.Duration {
	return time.Duration(config.Cfg.CacheTTLSeconds) * time.Second
}

func cacheValid(ts time.Time) bool {
	return time.Since(ts) < cacheTTL()
}

func invalidateCategories() {
	categoriesCacheMu.Lock()
	categoriesCache = nil
	categoriesCacheMu.Unlock()
}

func invalidateSubjects() {
	subjectsCacheMu.Lock()
	subjectsCache = map[string]*cacheEntry[models.Subject]{}
	subjectsCacheMu.Unlock()
}

func invalidateFiles(subjectID *int) {
	// Cache keys include the scope; clearing all entries is the safest and
	// simplest invalidation after a write in any bot database.
	filesCacheMu.Lock()
	filesCache = map[string]*cacheEntry[models.File]{}
	filesCacheMu.Unlock()
}

func itoa(n int) string {
	return bson.Raw{}.String() // placeholder, will use strconv below
}

// ── Categories ──────────────────────────────────────────────

// GetCategories returns all categories, sorted by order.
func GetCategories(ctx context.Context) ([]models.Category, error) {
	// Scoped bot databases intentionally bypass the legacy process-wide cache.
	// This prevents one bot's category list from being returned to another bot.
	useCache := !db.IsScoped(ctx)
	if useCache {
		categoriesCacheMu.RLock()
		if categoriesCache != nil && cacheValid(categoriesCache.ts) {
			defer categoriesCacheMu.RUnlock()
			return categoriesCache.data, nil
		}
		categoriesCacheMu.RUnlock()

		categoriesCacheMu.Lock()
		defer categoriesCacheMu.Unlock()

		if categoriesCache != nil && cacheValid(categoriesCache.ts) {
			return categoriesCache.data, nil
		}
	}

	col := db.ColScoped(ctx, "categories")
	opts := options.Find().SetSort(bson.D{{Key: "order", Value: 1}})
	cursor, err := col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var rows []models.Category
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}

	// Seed default categories if none exist
	if len(rows) == 0 {
		for idx, name := range config.FixedCategoryNames {
			catID, err := db.GetNextID(ctx, "categories")
			if err != nil {
				return nil, err
			}
			_, err = col.InsertOne(ctx, bson.M{
				"cat_id": catID,
				"name":   name,
				"order":  idx + 1,
			})
			if err != nil {
				return nil, err
			}
		}
		cursor, err = col.Find(ctx, bson.M{}, opts)
		if err != nil {
			return nil, err
		}
		rows = nil
		if err := cursor.All(ctx, &rows); err != nil {
			return nil, err
		}
	}

	if useCache {
		categoriesCache = &cacheEntry[models.Category]{data: rows, ts: time.Now()}
	}
	return rows, nil
}

// GetCategoryByID returns a single category by its cat_id.
func GetCategoryByID(ctx context.Context, catID int) (*models.Category, error) {
	cats, err := GetCategories(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range cats {
		if c.CatID == catID {
			return &c, nil
		}
	}
	// Fallback to DB
	var cat models.Category
	err = db.ColScoped(ctx, "categories").FindOne(ctx, bson.M{"cat_id": catID}).Decode(&cat)
	if err != nil {
		return nil, err
	}
	return &cat, nil
}

// CreateCategory creates a new category.
func CreateCategory(ctx context.Context, name string) (int, error) {
	col := db.ColScoped(ctx, "categories")
	catID, err := db.GetNextID(ctx, "categories")
	if err != nil {
		return 0, err
	}

	// Find the last order
	opts := options.FindOne().SetSort(bson.D{{Key: "order", Value: -1}})
	var last models.Category
	order := 1
	if err := col.FindOne(ctx, bson.M{}, opts).Decode(&last); err == nil {
		order = last.Order + 1
	}

	_, err = col.InsertOne(ctx, bson.M{"cat_id": catID, "name": name, "order": order})
	if err != nil {
		return 0, err
	}
	invalidateCategories()
	return catID, nil
}

// DeleteCategory deletes a category and all its subjects and files.
func DeleteCategory(ctx context.Context, catID int) error {
	subs, err := GetSubjects(ctx, &catID)
	if err != nil {
		return err
	}
	for _, s := range subs {
		if err := DeleteSubject(ctx, s.SubID); err != nil {
			log.Printf("Warning: failed to delete subject %d: %v", s.SubID, err)
		}
	}
	_, err = db.ColScoped(ctx, "categories").DeleteOne(ctx, bson.M{"cat_id": catID})
	invalidateCategories()
	return err
}

// ── Subjects ────────────────────────────────────────────────

// GetSubjects returns subjects, optionally filtered by categoryID.
func GetSubjects(ctx context.Context, categoryID *int) ([]models.Subject, error) {
	key := db.ScopeKey(ctx) + ":all"
	q := bson.M{}
	if categoryID != nil {
		key = db.ScopeKey(ctx) + ":" + intToStr(*categoryID)
		q = bson.M{"category_id": *categoryID}
	}

	subjectsCacheMu.RLock()
	if c, ok := subjectsCache[key]; ok && cacheValid(c.ts) {
		defer subjectsCacheMu.RUnlock()
		return c.data, nil
	}
	subjectsCacheMu.RUnlock()

	subjectsCacheMu.Lock()
	defer subjectsCacheMu.Unlock()

	if c, ok := subjectsCache[key]; ok && cacheValid(c.ts) {
		return c.data, nil
	}

	col := db.ColScoped(ctx, "subjects")
	opts := options.Find().SetSort(bson.D{{Key: "order", Value: 1}, {Key: "name", Value: 1}})
	cursor, err := col.Find(ctx, q, opts)
	if err != nil {
		return nil, err
	}
	var rows []models.Subject
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}

	// Backfill missing order fields
	needsOrder := false
	for _, r := range rows {
		if r.Order == 0 {
			needsOrder = true
			break
		}
	}
	if needsOrder && len(rows) > 0 {
		opts2 := options.Find().SetSort(bson.D{{Key: "name", Value: 1}})
		cursor2, err := col.Find(ctx, q, opts2)
		if err != nil {
			return rows, nil
		}
		var sorted []models.Subject
		if err := cursor2.All(ctx, &sorted); err != nil {
			return rows, nil
		}
		for idx, r := range sorted {
			_, _ = col.UpdateOne(ctx,
				bson.M{"sub_id": r.SubID},
				bson.M{"$set": bson.M{"order": idx + 1}},
			)
		}
		cursor3, err := col.Find(ctx, q, opts)
		if err == nil {
			rows = nil
			_ = cursor3.All(ctx, &rows)
		}
	}

	if rows == nil {
		rows = []models.Subject{}
	}
	subjectsCache[key] = &cacheEntry[models.Subject]{data: rows, ts: time.Now()}
	return rows, nil
}

// GetSubjectByID returns a single subject by its sub_id.
func GetSubjectByID(ctx context.Context, subID int) (*models.Subject, error) {
	var sub models.Subject
	err := db.ColScoped(ctx, "subjects").FindOne(ctx, bson.M{"sub_id": subID}).Decode(&sub)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// CreateSubject creates a new subject under a category.
func CreateSubject(ctx context.Context, name string, categoryID int) (int, error) {
	col := db.ColScoped(ctx, "subjects")
	subID, err := db.GetNextID(ctx, "subjects")
	if err != nil {
		return 0, err
	}

	q := bson.M{"category_id": categoryID}
	opts := options.FindOne().SetSort(bson.D{{Key: "order", Value: -1}})
	var last models.Subject
	order := 1
	if err := col.FindOne(ctx, q, opts).Decode(&last); err == nil {
		order = last.Order + 1
	}

	_, err = col.InsertOne(ctx, bson.M{
		"sub_id":      subID,
		"name":        name,
		"category_id": categoryID,
		"order":       order,
	})
	if err != nil {
		return 0, err
	}
	invalidateSubjects()
	invalidateCategories()
	return subID, nil
}

// DeleteSubject deletes a subject and all its files.
func DeleteSubject(ctx context.Context, subID int) error {
	_, _ = db.ColScoped(ctx, "files").DeleteMany(ctx, bson.M{"subject_id": subID})
	invalidateFiles(&subID)
	_, err := db.ColScoped(ctx, "subjects").DeleteOne(ctx, bson.M{"sub_id": subID})
	invalidateSubjects()
	invalidateCategories()
	return err
}

// ── Files ───────────────────────────────────────────────────

// GetFiles returns all files for a subject, sorted by order.
func GetFiles(ctx context.Context, subjectID int) ([]models.File, error) {
	key := db.ScopeKey(ctx) + ":" + intToStr(subjectID)

	filesCacheMu.RLock()
	if c, ok := filesCache[key]; ok && cacheValid(c.ts) {
		defer filesCacheMu.RUnlock()
		return c.data, nil
	}
	filesCacheMu.RUnlock()

	filesCacheMu.Lock()
	defer filesCacheMu.Unlock()

	if c, ok := filesCache[key]; ok && cacheValid(c.ts) {
		return c.data, nil
	}

	col := db.ColScoped(ctx, "files")
	q := bson.M{"subject_id": subjectID}
	opts := options.Find().SetSort(bson.D{{Key: "order", Value: 1}, {Key: "name", Value: 1}})
	cursor, err := col.Find(ctx, q, opts)
	if err != nil {
		return nil, err
	}
	var rows []models.File
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}

	// Backfill missing order fields
	needsOrder := false
	for _, r := range rows {
		if r.Order == 0 {
			needsOrder = true
			break
		}
	}
	if needsOrder && len(rows) > 0 {
		opts2 := options.Find().SetSort(bson.D{{Key: "name", Value: 1}})
		cursor2, err := col.Find(ctx, q, opts2)
		if err != nil {
			return rows, nil
		}
		var sorted []models.File
		if err := cursor2.All(ctx, &sorted); err != nil {
			return rows, nil
		}
		for idx, r := range sorted {
			_, _ = col.UpdateOne(ctx,
				bson.M{"file_id": r.FileID},
				bson.M{"$set": bson.M{"order": idx + 1}},
			)
		}
		cursor3, err := col.Find(ctx, q, opts)
		if err == nil {
			rows = nil
			_ = cursor3.All(ctx, &rows)
		}
	}

	if rows == nil {
		rows = []models.File{}
	}
	filesCache[key] = &cacheEntry[models.File]{data: rows, ts: time.Now()}
	return rows, nil
}

// GetFileRow returns a file with its subject name.
func GetFileRow(ctx context.Context, fileID int) (*models.FileRow, error) {
	var f models.File
	err := db.ColScoped(ctx, "files").FindOne(ctx, bson.M{"file_id": fileID}).Decode(&f)
	if err != nil {
		return nil, err
	}
	subName := "غير معروف"
	var sub models.Subject
	if err := db.ColScoped(ctx, "subjects").FindOne(ctx, bson.M{"sub_id": f.SubjectID}).Decode(&sub); err == nil {
		subName = sub.Name
	}
	return &models.FileRow{File: f, SubjectName: subName}, nil
}

// SaveFile inserts a new file into the database.
func SaveFile(ctx context.Context, name, telegramFileID, fileType string, subjectID int, messageID int, fileSize int64) (int, error) {
	col := db.ColScoped(ctx, "files")
	fileID, err := db.GetNextID(ctx, "files")
	if err != nil {
		return 0, err
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "order", Value: -1}})
	var last models.File
	order := 1
	if err := col.FindOne(ctx, bson.M{"subject_id": subjectID}, opts).Decode(&last); err == nil {
		order = last.Order + 1
	}

	_, err = col.InsertOne(ctx, bson.M{
		"file_id":          fileID,
		"name":             name,
		"telegram_file_id": telegramFileID,
		"file_type":        fileType,
		"subject_id":       subjectID,
		"message_id":       messageID,
		"file_size":        fileSize,
		"order":            order,
		"upload_date":      time.Now().UTC(),
	})
	if err != nil {
		return 0, err
	}
	invalidateFiles(&subjectID)
	return fileID, nil
}

// DeleteFile deletes a single file.
func DeleteFile(ctx context.Context, fileID int) error {
	var f models.File
	err := db.ColScoped(ctx, "files").FindOne(ctx, bson.M{"file_id": fileID}).Decode(&f)
	_, delErr := db.ColScoped(ctx, "files").DeleteOne(ctx, bson.M{"file_id": fileID})
	if err == nil {
		invalidateFiles(&f.SubjectID)
	}
	return delErr
}

// ── Move operations ─────────────────────────────────────────

func swapAndRenormalize(ctx context.Context, collection string, rows []bson.M, keyField, direction string, targetKey int) (bool, error) {
	idx := -1
	for i, r := range rows {
		if toInt(r[keyField]) == targetKey {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false, nil
	}
	newIdx := idx - 1
	if direction == "down" {
		newIdx = idx + 1
	}
	if newIdx < 0 || newIdx >= len(rows) {
		return false, nil
	}
	rows[idx], rows[newIdx] = rows[newIdx], rows[idx]
	col := db.ColScoped(ctx, collection)
	for i, r := range rows {
		if toInt(r["order"]) != i+1 {
			_, err := col.UpdateOne(ctx,
				bson.M{keyField: r[keyField]},
				bson.M{"$set": bson.M{"order": i + 1}},
			)
			if err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int32:
		return int(val)
	case int64:
		return int(val)
	case float64:
		return int(val)
	default:
		return 0
	}
}

// MoveCategory moves a category up or down.
func MoveCategory(ctx context.Context, catID int, direction string) (bool, error) {
	col := db.ColScoped(ctx, "categories")
	opts := options.Find().SetSort(bson.D{{Key: "order", Value: 1}, {Key: "name", Value: 1}})
	cursor, err := col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return false, err
	}
	var rows []bson.M
	if err := cursor.All(ctx, &rows); err != nil {
		return false, err
	}
	moved, err := swapAndRenormalize(ctx, "categories", rows, "cat_id", direction, catID)
	if moved {
		invalidateCategories()
	}
	return moved, err
}

// MoveSubject moves a subject up or down within its category.
func MoveSubject(ctx context.Context, subID int, direction string) (bool, error) {
	sub, err := GetSubjectByID(ctx, subID)
	if err != nil {
		return false, err
	}
	col := db.ColScoped(ctx, "subjects")
	opts := options.Find().SetSort(bson.D{{Key: "order", Value: 1}, {Key: "name", Value: 1}})
	cursor, err := col.Find(ctx, bson.M{"category_id": sub.CategoryID}, opts)
	if err != nil {
		return false, err
	}
	var rows []bson.M
	if err := cursor.All(ctx, &rows); err != nil {
		return false, err
	}
	moved, err := swapAndRenormalize(ctx, "subjects", rows, "sub_id", direction, subID)
	if moved {
		invalidateSubjects()
	}
	return moved, err
}

// MoveFile moves a file up or down within its subject.
func MoveFile(ctx context.Context, fileID int, direction string) (bool, error) {
	var f models.File
	err := db.ColScoped(ctx, "files").FindOne(ctx, bson.M{"file_id": fileID}).Decode(&f)
	if err != nil {
		return false, err
	}
	col := db.ColScoped(ctx, "files")
	opts := options.Find().SetSort(bson.D{{Key: "order", Value: 1}, {Key: "name", Value: 1}})
	cursor, err := col.Find(ctx, bson.M{"subject_id": f.SubjectID}, opts)
	if err != nil {
		return false, err
	}
	var rows []bson.M
	if err := cursor.All(ctx, &rows); err != nil {
		return false, err
	}
	moved, err := swapAndRenormalize(ctx, "files", rows, "file_id", direction, fileID)
	if moved {
		invalidateFiles(&f.SubjectID)
	}
	return moved, err
}

// ── Stats ───────────────────────────────────────────────────

// GetAllUsersCount returns total user count.
func GetAllUsersCount(ctx context.Context) (int64, error) {
	return db.ColScoped(ctx, "users").CountDocuments(ctx, bson.M{})
}

// GetAllFilesCount returns total file count.
func GetAllFilesCount(ctx context.Context) (int64, error) {
	return db.ColScoped(ctx, "files").CountDocuments(ctx, bson.M{})
}

// GetAllSubjectsCount returns total subject count.
func GetAllSubjectsCount(ctx context.Context) (int64, error) {
	return db.ColScoped(ctx, "subjects").CountDocuments(ctx, bson.M{})
}

// IncrementDownloads increments the download counter for a file.
func IncrementDownloads(ctx context.Context, fileID int) error {
	_, err := db.ColScoped(ctx, "files").UpdateOne(ctx,
		bson.M{"file_id": fileID},
		bson.M{"$inc": bson.M{"downloads": 1}},
	)
	return err
}

// GetTotalDownloads returns the sum of all file downloads.
func GetTotalDownloads(ctx context.Context) (int64, error) {
	pipeline := bson.A{
		bson.M{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$downloads"}}},
	}
	cursor, err := db.ColScoped(ctx, "files").Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return 0, err
	}
	if len(results) == 0 {
		return 0, nil
	}
	return int64(toInt(results[0]["total"])), nil
}

// TopFile holds info about a top-downloaded file.
type TopFile struct {
	Name        string
	Downloads   int
	SubjectName string
}

// GetTopFiles returns the most downloaded files.
func GetTopFiles(ctx context.Context, limit int) ([]TopFile, error) {
	col := db.ColScoped(ctx, "files")
	opts := options.Find().
		SetSort(bson.D{{Key: "downloads", Value: -1}}).
		SetLimit(int64(limit))
	cursor, err := col.Find(ctx, bson.M{"downloads": bson.M{"$gt": 0}}, opts)
	if err != nil {
		return nil, err
	}
	var rows []models.File
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	var results []TopFile
	for _, r := range rows {
		subName := "غير معروف"
		var sub models.Subject
		if err := db.ColScoped(ctx, "subjects").FindOne(ctx, bson.M{"sub_id": r.SubjectID}).Decode(&sub); err == nil {
			subName = sub.Name
		}
		results = append(results, TopFile{
			Name:        r.Name,
			Downloads:   r.Downloads,
			SubjectName: subName,
		})
	}
	return results, nil
}

// intToStr converts an int to string for cache keys.
func intToStr(n int) string {
	return strconv.Itoa(n)
}
