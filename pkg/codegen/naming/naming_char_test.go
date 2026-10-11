// pkg/codegen/naming/naming_char_test.go
package naming

import "testing"

// 特征化测试：钉住当前分词行为，重构 Snake 时防静默漂移。
// 注意 V2/IPv4 这类大小写边界当前会插下划线（order_item_v_2）——
// 这是**现状**而非期望值；改行为必须先改本测试并全量评估生成物 diff。
func TestSnake_Characterized(t *testing.T) {
	cases := map[string]string{
		"APIKey":      "api_key",
		"OrderItemV2": "order_item_v_2",
		"V2Ray":       "v_2_ray",
		"UserID2":     "user_id_2",
		"HTTPServer":  "http_server",
		"IPv4Addr":    "i_pv_4_addr",
		"tenantID":    "tenant_id",
	}
	for in, want := range cases {
		if got := Snake(in); got != want {
			t.Errorf("Snake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEntityName_Characterized(t *testing.T) {
	if got := EntityName("APIKey"); got != "Apikey" {
		t.Errorf("EntityName(APIKey) = %q, want %q", got, "Apikey")
	}
}
