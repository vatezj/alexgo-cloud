package model

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAutoMigrateAll(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Room{}, &RoomUser{}, &RoomLog{}, &Record{}, &Friend{}, &FriendBlack{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	var cols []string
	if err := db.Raw(`SELECT name FROM pragma_table_info('scorecard_room')`).Scan(&cols).Error; err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"room_code": true, "tea_pool_total": true, "tea_auto_enabled": true,
		"pending_apply_round_no": true, "version": true, "tenant_id": true}
	for _, c := range cols {
		delete(want, c)
	}
	if len(want) != 0 {
		t.Fatalf("room 缺列: %v（实际列 %v）", want, cols)
	}
}

// openTestDB 内存 sqlite 上全量 AutoMigrate 6 表（与 TestAutoMigrateAll 同一批模型）。
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Room{}, &RoomUser{}, &RoomLog{}, &Record{}, &Friend{}, &FriendBlack{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

// tableCols 取某表当前列名清单（sqlite pragma）。
func tableCols(t *testing.T, db *gorm.DB, table string) []string {
	t.Helper()
	var cols []string
	if err := db.Raw(`SELECT name FROM pragma_table_info(?)`, table).Scan(&cols).Error; err != nil {
		t.Fatalf("pragma_table_info(%s): %v", table, err)
	}
	return cols
}

// allTables 6 表清单（与迁移 DDL、TableName 对齐；模块前缀 scorecard_）。
var allTables = []string{"scorecard_room", "scorecard_room_user", "scorecard_room_log", "scorecard_record", "scorecard_friend", "scorecard_friend_black"}

// TestTenantStampColumns 钉住 spec 红线：全部 6 表带 tenant_id + created_at/updated_at。
func TestTenantStampColumns(t *testing.T) {
	db := openTestDB(t)
	for _, table := range allTables {
		got := map[string]bool{}
		for _, c := range tableCols(t, db, table) {
			got[c] = true
		}
		for _, want := range []string{"tenant_id", "created_at", "updated_at"} {
			if !got[want] {
				t.Errorf("%s 缺列 %s（实际列 %v）", table, want, tableCols(t, db, table))
			}
		}
	}
}

// TestUniqueAndCompositeIndexes 钉住唯一键与复合索引由 gorm 标签落库，
// 且名字与迁移 DDL 的 uk_*/idx_* 一一对应（同构 = 同名同列）。
func TestUniqueAndCompositeIndexes(t *testing.T) {
	db := openTestDB(t)
	cases := []struct {
		table  string
		index  string
		unique bool
	}{
		{"scorecard_room", "uk_room_code", true},
		{"scorecard_room_user", "uk_room_user", true},
		{"scorecard_friend", "uk_friend", true},
		{"scorecard_friend_black", "uk_friend_black", true},
		{"scorecard_room_log", "idx_room_log_room_created", false},
		{"scorecard_record", "idx_record_user_created", false},
		{"scorecard_record", "idx_record_room", false},
	}
	for _, c := range cases {
		var sql string
		if err := db.Raw(`SELECT sql FROM sqlite_master WHERE type='index' AND name=?`, c.index).Scan(&sql).Error; err != nil {
			t.Fatalf("查询索引 %s: %v", c.index, err)
		}
		if sql == "" {
			t.Errorf("%s 缺索引 %s", c.table, c.index)
			continue
		}
		isUnique := strings.HasPrefix(sql, "CREATE UNIQUE INDEX")
		if c.unique && !isUnique {
			t.Errorf("%s.%s 应为唯一索引，实际建表语句: %s", c.table, c.index, sql)
		}
		if !c.unique && isUnique {
			t.Errorf("%s.%s 不应为唯一索引，实际建表语句: %s", c.table, c.index, sql)
		}
	}
}

// migrationDDLPath 迁移 up.sql（相对本包目录，见 ../migrations/）。
const migrationDDLPath = "../migrations/20261020000001_scorecard_tables.up.sql"

// parseMigrationDDL 解析迁移 up.sql：每张表的列名清单（PRIMARY KEY 之前、行首反引号者）。
func parseMigrationDDL(t *testing.T) map[string][]string {
	t.Helper()
	raw, err := os.ReadFile(migrationDDLPath)
	if err != nil {
		t.Fatalf("读迁移 DDL: %v", err)
	}
	re := regexp.MustCompile("(?s)CREATE TABLE IF NOT EXISTS `([a-z_]+)` \\((.*?)\\) ENGINE")
	matches := re.FindAllStringSubmatch(string(raw), -1)
	if len(matches) != len(allTables) {
		t.Fatalf("迁移 DDL 应含 %d 张表，实际解析到 %d", len(allTables), len(matches))
	}
	out := map[string][]string{}
	for _, m := range matches {
		table, body := m[1], m[2]
		var cols []string
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "PRIMARY KEY") {
				break
			}
			if !strings.HasPrefix(line, "`") {
				continue
			}
			name := strings.TrimPrefix(line, "`")
			end := strings.Index(name, "`")
			if end <= 0 {
				t.Fatalf("迁移 DDL %s 列行无法解析: %q", table, line)
			}
			cols = append(cols, name[:end])
		}
		out[table] = cols
	}
	return out
}

// TestMigrationDDLMatchesModels 迁移 DDL 列集合与模型 AutoMigrate 结果双向同构：
// DDL 有而模型缺 → 迁移建的列查不出来；模型有而 DDL 缺 → 生产表缺字段。
func TestMigrationDDLMatchesModels(t *testing.T) {
	db := openTestDB(t)
	ddlCols := parseMigrationDDL(t)
	for table, wantCols := range ddlCols {
		gotCols := tableCols(t, db, table)
		want := map[string]bool{}
		for _, c := range wantCols {
			want[c] = true
		}
		got := map[string]bool{}
		for _, c := range gotCols {
			got[c] = true
		}
		for c := range want {
			if !got[c] {
				t.Errorf("%s: 迁移 DDL 有列 %s 而模型缺（DDL %v / 模型 %v）", table, c, wantCols, gotCols)
			}
		}
		for c := range got {
			if !want[c] {
				t.Errorf("%s: 模型有列 %s 而迁移 DDL 缺（DDL %v / 模型 %v）", table, c, wantCols, gotCols)
			}
		}
	}
}
