
# Project TODO

- [x] إصلاح إرسال الملفات المصنفة كصور كصور Telegram فعلية بدلاً من مستندات
- [x] اختبار تحميل الصور مع الحفاظ على المستندات والفيديو والصوت
- [x] كتابة README احترافي شامل للتشغيل والبنية والإعدادات وواجهات API
- [x] مراجعة وإصلاح الأخطاء الحالية في البوت مع اختبارات regression
- [x] إضافة ميزة تحويل الملفات بين صيغ مدعومة وآمنة
- [x] ربط التحويل بتدفق Telegram مع حدود الحجم والمهلة والتنظيف
- [x] توثيق الصيغ المدعومة وأمثلة الاستخدام في README
- [x] إضافة مخطط معماري احترافي ومصدر Mermaid إلى توثيق البوت
- [x] إزالة ميزة تحويل الملفات بالكامل وأمر /convert وFFmpeg وإعداداته
- [x] تصميم وتنفيذ بحث متقدم داخل الأرشيف
- [x] إضافة فلاتر النوع والتاريخ والتصنيف والمادة وترتيب النتائج
- [x] ربط البحث بواجهة Telegram وAPI مع اختبارات وتوثيق
- [x] استبدال محتوى المستودع الهدف بسورس Go مع الحفاظ على الـRelease القديم وإنشاء Release جديد
- [x] إضافة GNU GPL v3 وتحديث جميع المستودعات وملفات Releases
- [x] إنشاء أيقونات وهوية بصرية مرتبة للمشروع
- [x] تجهيز منشور Instagram احترافي بمقاس مربع
- [x] رفع الأصول وتحديث README في المستودعات
- [x] إنشاء موكاب Repository Social Preview للمشروع وإضافته إلى كل ريبو
- [x] إزالة البحث المتقدم من البوت والـAPI والتوثيق
- [x] إصلاح إرسال الملفات المصنفة كصور لإرسالها كـDocument
- [x] مزامنة الإصلاحات مع المستودع الهدف وإنشاء Release
- [x] إصلاح خمول البوت وتقوية health endpoint وتهيئة التشغيل المستمر
- [x] توثيق إعدادات Koyeb المطلوبة للتشغيل المستمر
- [x] مزامنة الإصلاح مع المستودع الهدف وإنشاء Release
- [x] تصميم Bot Factory متعدد البوتات مع إدارة دورة حياة البوتات
- [x] تنفيذ API v2 لإدارة البوتات والتوكنات والصلاحيات
- [x] إضافة موزع حمل ذكي مع health checks وcircuit breaker وقياس الضغط
- [x] إضافة اختبارات وتوثيق وخطة ترحيل آمنة
- [x] إكمال دمج Bot Factory مع البوت الرئيسي وصلاحيات الإدارة
- [x] تقوية دورة حياة العمال والإرسال الموزع في API v2
- [x] اختبار الدمج ومزامنة المستودعين وإنشاء Release موحد
- [x] إنشاء namespace وfolder مستقل لكل بوت مع قناة Telegram Storage مشتركة
- [x] ربط Worker وAPI بعزل البوت ومنع تسرب البيانات بين البوتات
- [x] إضافة اختبارات وتوثيق ومزامنة Release للعزل
- [x] جعل البوت الرئيسي Storage Gateway لإرسال ملفات قناة التخزين المشتركة نيابة عن البوتات المُدارة
- [x] ربط namespace البوت بمجلد منطقي مستقل دون اشتراط صلاحية قناة للبوت الفرعي

# Bot Factory Production Expansion

- [x] إضافة Storage Gateway Queue دائمة في MongoDB لإرسال الملفات عبر البوت الرئيسي
- [x] إضافة Retry ذكي مع exponential backoff وdead-letter status لطلبات التخزين
- [x] إضافة حدود مستقلة لكل بوت للاستخدام والملفات والطلبات
- [x] إضافة usage counters وواجهات إحصائيات لكل namespace
- [x] إضافة health dashboard/API للبوتات مع latency وerrors وqueue depth
- [x] إضافة أدوار مستقلة لكل بوت: owner وadmin وeditor وviewer
- [x] إضافة audit log موحد لكل تغييرات الإدارة داخل namespace البوت
- [x] إضافة Maintenance mode مستقل لكل بوت
- [x] إضافة backup وrestore لقاعدة بوت واحد دون التأثير على البقية
- [x] إضافة تدوير آمن لتوكن البوت مع الحفاظ على database namespace
- [x] إضافة API keys مستقلة بصلاحيات محددة لكل بوت
- [x] إضافة AI Gateway scoped مع فهرسة وتصنيف وتلخيص خاص بكل بوت
- [x] إضافة اختبارات وتوثيق ومزامنة Release لكل ميزات التوسعة

# Parent Bot Auto-Scaling Database Controller

- [x] إضافة مراقب مركزي من البوت الأب لقواعد البوتات الأبناء
- [x] إضافة مؤشرات توسعة وحدود قابلة للضبط لكل قاعدة بوت
- [x] إضافة expansion plans وleases لمنع تشغيل توسعة مزدوجة
- [x] إضافة migrations idempotent وbackfill تدريجي دون إيقاف البوت
- [x] إضافة auto-provision للـcollections والفهارس عند إنشاء بوت
- [x] إضافة API لحالة التوسعة وآخر migration لكل بوت
- [x] إضافة اختبارات وتوثيق ومزامنة Release للتوسعة التلقائية

# Horizontal Sharding and Online Rebalancing

- [ ] إضافة سجل عقد أو قواعد MongoDB القابلة للتوسعة تحت تحكم البوت الأب
- [ ] إضافة shard map وvirtual shards مع إبقاء namespace كل بوت معزولاً
- [ ] إضافة Shard Router للقراءة والكتابة مع fallback آمن
- [ ] إضافة online rebalancer بنقل chunked وdual-write وchecksum
- [ ] إضافة cutover ذري وrollback دون إيقاف البوت
- [ ] توزيع virtual shards تلقائياً عند إضافة قاعدة جديدة
- [ ] إضافة API لحالة العقد والتوزيع وعمليات النقل
- [ ] إضافة اختبارات وتوثيق ومزامنة Release للتوسعة الأفقية

# Separate MongoDB Cluster Distribution

- [x] إضافة Cluster Registry مشفر داخل قاعدة البوت الأب
- [x] إضافة فحص اتصال وصحة لكل MongoDB Cluster جديد
- [ ] إضافة shard map مستقل لكل bot namespace
- [ ] إضافة Router متعدد الاتصالات مع fallback دون خلط بيانات البوتات
- [ ] إضافة dual-write ونقل chunks مع checksum وcutover ذري
- [ ] إضافة rollback وإعادة محاولة لعمليات النقل الفاشلة
- [ ] إضافة API لتسجيل وإزالة ومراقبة Clusters
- [ ] إضافة اختبارات وتوثيق ومزامنة Release للتوزيع متعدد الـClusters

# Telegram Parent DB Control Panel

- [x] إضافة زر أو أمر `/dbpanel` في لوحة البوت الأب
- [x] إضافة تدفق إدخال اسم وMongo URI في محادثة خاصة مع المالك
- [x] حذف رسالة Mongo URI بعد قراءتها ومنع تسجيلها في logs أو API
- [x] إضافة عرض Clusters وحالتها وتفعيلها وتعطيلها وإزالتها من Telegram
- [x] إضافة اختبار اتصال قبل حفظ Cluster وتشفير بياناته
- [ ] ربط إضافة Cluster بتوزيع virtual shards وإعادة التوازن الآمن
- [x] إضافة اختبارات وتوثيق ومزامنة Release لتدفق لوحة التحكم

# Near-zero Downtime Migration

- [x] إضافة migration job خاص بكل bot مع source وtarget وprogress
- [x] نسخ بيانات bot namespace على دفعات مع checksum
- [x] إيقاف worker المستهدف خلال cutover فقط
- [x] نسخ delta الأخير قبل تبديل route
- [x] إبقاء المصدر كنسخة rollback وعدم حذفه تلقائياً
- [x] إضافة أوامر `/migratebot` و`/migrationstatus` للوحة البوت الأب
- [ ] اختبار وتوثيق النقل بين Clusters منفصلة ومزامنة Release

# Lightweight Group Bot and Unified API

- [x] إضافة group namespace مستقل لكل مجموعة ولكل بوت
- [ ] إضافة إعدادات وصلاحيات المجموعة مع owner وadmin وmoderator وmember
- [ ] دعم أوامر المجموعة مع التحقق من صلاحيات Telegram
- [x] ربط وظائف الأرشيف والملفات وAI عبر API v2 موحد يحمل bot_id وchat_id
- [ ] إضافة group rate limits وanti-flood وحدود الملفات
- [ ] تقليل استهلاك الذاكرة عبر bounded queues وstreaming وعدم تخزين media bytes
- [ ] تحسين MongoDB عبر connection pool محدود وفهارس group scoped وقراءة projection
- [ ] إضافة cache خفيف TTL مع منع تسرب بيانات مجموعة إلى أخرى
- [ ] إضافة health وmetrics للأداء على مستوى البوت والمجموعة
- [ ] إضافة اختبارات وتوثيق ومزامنة Release لدعم المجموعات

# Personal Vault and Subject Subscriptions

- [x] إضافة Personal Vault مع عزل ملفات المستخدم حسب bot_id وuser_id
- [x] إضافة اشتراك المستخدم في مادة محددة مع منع التكرار
- [x] ربط رفع ملف جديد بإشعارات المشتركين في المادة
- [x] استخدام queue وحدود دفعة لمنع ضغط Telegram أو MongoDB
- [x] إضافة idempotency وسجل إشعار يمنع إرسال التنبيه مرتين
- [x] إضافة أوامر Telegram للاشتراك وإلغاء الاشتراك وعرض الاشتراكات
- [x] إضافة اختبارات وتوثيق لتدفق Vault والاشتراكات

# Child Bot Update Routing and Role-aware Welcome

- [x] ضمان تسجيل أوامر البوتات الفرعية العامة عند بدء worker
- [x] التحقق من تمرير callback والرسائل والملفات من البوت الفرعي إلى نفس handler
- [x] إخفاء Bot Factory عن جميع المستخدمين والبوتات الفرعية
- [x] إظهار زر لوحة التحكم داخل الترحيب للمشرفين فقط
- [x] إضافة اختبارات visibility وصلاحيات مالك البوت الأب
- [x] مزامنة الإصلاحات والتوثيق إلى المستودع الثانوي

# Child Bot Delivery and Responsiveness

- [x] تشخيص سبب توقف البوت الفرعي بعد رسالة الترحيب مع فحص polling وTelegram updates
- [x] إصلاح تمرير الأوامر والـcallbacks والملفات للبوتات الفرعية
- [x] إضافة عملية تسليم بوت لشخص محدد من البوت الأب
- [x] إضافة تعيين أدمن للمستلم بصلاحيات محددة داخل namespace البوت
- [x] حماية التسليم من المستخدم غير المصرح ومن تكرار التنفيذ
- [x] إضافة اختبارات وتوثيق ومزامنة المستودع الثانوي

- [x] إضافة أمر `/id` لتسهيل أخذ Telegram user ID عند تسليم البوت

- [x] منع تشغيل polling متزامن لنفس Telegram bot عبر أكثر من instance باستخدام distributed lease
- [x] جعل health check لا يفتح اتصالاً متعارضاً مع polling وإضافة تشخيص واضح لـ 409 Conflict

# Persistent Conflict Regression

- [x] تحليل سجل التشغيل الجديد بعد نشر lease والتحقق من commit وعدد instances الفعلي
- [x] تحديد العملية الثانية التي تستخدم BOT_TOKEN أو اكتشاف duplicate workers داخل نفس process
- [x] منع التعارض النهائي واختبار أن child bot يستقبل الأوامر والملفات
- [x] توثيق خطوات تنظيف Webhook وإيقاف الخدمة القديمة والتحقق من Telegram getUpdates

# Child Bot Not Responding Regression

- [ ] تحديد البوت الفرعي المتأثر وحالة worker الخاصة به من السجل
- [ ] التحقق من عدم وجود duplicate polling أو فشل lease أو Webhook
- [ ] إصلاح سبب عدم استقبال تحديثات child bot
- [ ] اختبار `/start` والأوامر والملفات ومزامنة الإصلاح

# Full Production Audit

- [ ] جرد أخطاء compile وruntime وقراءة مسارات startup وshutdown
- [ ] تدقيق polling والـworker lifecycle والـlease وتعارض 409
- [ ] تدقيق عزل bot namespace وcluster routing وعمليات migration
- [ ] تدقيق صلاحيات Bot Factory وhandoff وgroup وAPI keys
- [ ] تدقيق queue والتخزين والملفات والصور والإشعارات وPersonal Vault
- [ ] تدقيق الأسرار والإعدادات والـDocker/Koyeb والـresource limits
- [ ] إصلاح الأخطاء المؤكدة وإضافة اختبارات regression
- [ ] مزامنة نتائج التدقيق والتوثيق إلى المستودع الثانوي

- [x] إغلاق اتصالات MongoDB الخارجية عند shutdown لمنع تسريب الموارد
- [x] إضافة فحص اتصال MongoDB الخارجي واختبار cleanup

- [x] حماية قراءة client وcluster routes أثناء إغلاق MongoDB لمنع data race في shutdown

- [x] إضافة context timeout وحد أقصى لحجم تنزيل Telegram في مسار التحويل والملفات المساعدة
- [x] استبدال goroutines غير المحدودة في استقبال التحديثات بإدارة توازي محدودة

- [x] إصلاح archiveContextFromRequest ليستخدم lookup مباشر لـ bot_id ولا يفشل بعد أكثر من 100 بوت

# Child Bot Persistent Failure

- [x] إضافة trace مؤقت يحدد وصول update إلى worker وhandleUpdate وhandler
- [x] التحقق من أن آخر commit منشور فعلياً في بيئة التشغيل
- [x] إصلاح نقطة الفشل المثبتة فقط وإضافة اختبار لها

- [x] إضافة unique index على managed_bots.telegram_bot_id ومنع تكرار البوت عند تدوير Token

- [x] إعادة محاولة اكتساب lease للـchild والـparent بدلاً من حذف worker عند التعارض المؤقت أثناء rolling deploy
- [x] إعادة بدء polling تلقائياً بعد انتهاء النسخة القديمة أو تحرير lease

# Child Update Trace

- [ ] إضافة log آمن يثبت وصول تحديث child إلى polling وhandleUpdate
- [x] تسجيل سبب إسقاط التحديث إن كان quota أو نوع update أو handler
- [x] اختبار `/start` و`/id` وcallback بعد نشر trace
