package model

import "testing"

// 9 模型 TableName 必须与迁移 DDL 表名严格同构（Task 3 spec §5）。
func TestTableName(t *testing.T) {
	cases := []struct {
		model any
		want  string
	}{
		{Player{}, "fishgame_player"},
		{Session{}, "fishgame_session"},
		{Fish{}, "fishgame_fish"},
		{Dex{}, "fishgame_dex"},
		{TrackEvent{}, "fishgame_track_event"},
		{Sign{}, "fishgame_sign"},
		{Friend{}, "fishgame_friend"},
		{Config{}, "fishgame_config"},
		{Share{}, "fishgame_share"},
	}
	for _, c := range cases {
		var name string
		switch m := c.model.(type) {
		case Player:
			name = m.TableName()
		case Session:
			name = m.TableName()
		case Fish:
			name = m.TableName()
		case Dex:
			name = m.TableName()
		case TrackEvent:
			name = m.TableName()
		case Sign:
			name = m.TableName()
		case Friend:
			name = m.TableName()
		case Config:
			name = m.TableName()
		case Share:
			name = m.TableName()
		default:
			t.Fatalf("unhandled model type %T", m)
		}
		if name != c.want {
			t.Errorf("TableName() = %q, want %q", name, c.want)
		}
	}
}
