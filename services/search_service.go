package services

import (
	"context"
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"telegram-archive-bot/db"
	"telegram-archive-bot/models"
)

type FileSearchParams struct {
	Query      string
	CategoryID int
	SubjectID  int
	FileType   string
	Sort       string
	From       interface{}
	To         interface{}
	Page       int
	Limit      int
}

type FileSearchResult struct {
	Files []models.FileRow
	Total int
	Page  int
	Limit int
}

// SearchFiles searches file metadata and joined subject fields using bounded pagination.
func SearchFiles(ctx context.Context, params FileSearchParams) (FileSearchResult, error) {
	if params.Page < 0 {
		params.Page = 0
	}
	if params.Limit <= 0 || params.Limit > 20 {
		params.Limit = 10
	}
	match := bson.M{}
	and := bson.A{}
	if q := strings.TrimSpace(params.Query); q != "" {
		pattern := primitiveRegex(q)
		and = append(and, bson.M{"$or": bson.A{
			bson.M{"name": pattern},
			bson.M{"file_type": pattern},
			bson.M{"subject.name": pattern},
		}})
	}
	if params.CategoryID > 0 {
		and = append(and, bson.M{"subject.category_id": params.CategoryID})
	}
	if params.SubjectID > 0 {
		and = append(and, bson.M{"subject_id": params.SubjectID})
	}
	if ft := strings.TrimSpace(params.FileType); ft != "" {
		and = append(and, bson.M{"file_type": strings.ToLower(ft)})
	}
	date := bson.M{}
	if params.From != nil {
		date["$gte"] = params.From
	}
	if params.To != nil {
		date["$lte"] = params.To
	}
	if len(date) > 0 {
		and = append(and, bson.M{"upload_date": date})
	}
	if len(and) > 0 {
		match["$and"] = and
	}

	sortField := "upload_date"
	sortDirection := -1
	switch params.Sort {
	case "oldest":
		sortDirection = 1
	case "downloads":
		sortField = "downloads"
	case "name":
		sortField = "name"
		sortDirection = 1
	case "order":
		sortField = "order"
		sortDirection = 1
	}

	pipeline := mongoPipeline(match, sortField, sortDirection, params.Page*params.Limit, params.Limit)
	cursor, err := db.Col("files").Aggregate(ctx, pipeline)
	if err != nil {
		return FileSearchResult{}, err
	}
	defer cursor.Close(ctx)
	var rows []struct {
		Data  []models.FileRow `bson:"data"`
		Total []struct {
			Count int `bson:"count"`
		} `bson:"total"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return FileSearchResult{}, err
	}
	result := FileSearchResult{Files: []models.FileRow{}, Page: params.Page, Limit: params.Limit}
	if len(rows) == 1 {
		result.Files = rows[0].Data
		if len(rows[0].Total) == 1 {
			result.Total = rows[0].Total[0].Count
		}
	}
	return result, nil
}

func primitiveRegex(query string) primitive.Regex {
	return primitive.Regex{Pattern: regexp.QuoteMeta(query), Options: "i"}
}

func mongoPipeline(match bson.M, sortField string, sortDirection, skip, limit int) bson.A {
	return bson.A{
		bson.M{"$lookup": bson.M{"from": "subjects", "localField": "subject_id", "foreignField": "sub_id", "as": "subject"}},
		bson.M{"$unwind": bson.M{"path": "$subject", "preserveNullAndEmptyArrays": true}},
		bson.M{"$match": match},
		bson.M{"$facet": bson.M{
			"data": bson.A{
				bson.M{"$sort": bson.D{{Key: sortField, Value: sortDirection}, {Key: "file_id", Value: 1}}},
				bson.M{"$skip": skip},
				bson.M{"$limit": limit},
				bson.M{"$project": bson.M{"_id": 0, "file_id": 1, "name": 1, "telegram_file_id": 1, "file_type": 1, "subject_id": 1, "message_id": 1, "file_size": 1, "order": 1, "downloads": 1, "upload_date": 1, "subject_name": "$subject.name"}},
			},
			"total": bson.A{bson.M{"$count": "count"}},
		}},
	}
}
