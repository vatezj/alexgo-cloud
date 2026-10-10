// pkg/codegen/naming/naming_test.go
package naming_test

import (
	"testing"

	"alexGo-cloud/pkg/codegen/naming"
)

func TestEntityName(t *testing.T) {
	cases := map[string]string{
		"order_items": "OrderItem",
		"orders":      "Order",
		"order":       "Order",
		"addresses":   "Address",
		"categories":  "Category",
		"user_infos":  "UserInfo",
		"":            "",
	}
	for in, want := range cases {
		if got := naming.EntityName(in); got != want {
			t.Errorf("EntityName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToGoName(t *testing.T) {
	cases := map[string]string{
		"user_id":    "UserID",
		"identity":   "Identity", // 回归：不得出现 "IDentity"
		"order_no":   "OrderNo",
		"created_at": "CreatedAt",
		"url":        "URL",
		"json_data":  "JSONData",
		"api_key":    "APIKey",
		"":           "",
	}
	for in, want := range cases {
		if got := naming.ToGoName(in); got != want {
			t.Errorf("ToGoName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSnake(t *testing.T) {
	cases := map[string]string{
		"OrderItem":   "order_item",
		"UserID":      "user_id",
		"Order":       "order",
		"HTTPRequest": "http_request",
		"":            "",
	}
	for in, want := range cases {
		if got := naming.Snake(in); got != want {
			t.Errorf("Snake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlural(t *testing.T) {
	cases := map[string]string{
		"Order":     "orders",
		"OrderItem": "order_items",
		"Address":   "addresses",
	}
	for in, want := range cases {
		if got := naming.Plural(in); got != want {
			t.Errorf("Plural(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJSONName_Sensitive(t *testing.T) {
	if got := naming.JSONName("password"); got != "-" {
		t.Errorf("JSONName(password) = %q, want -", got)
	}
	if got := naming.JSONName("api_token"); got != "-" {
		t.Errorf("JSONName(api_token) = %q, want -", got)
	}
	if got := naming.JSONName("client_secret"); got != "-" {
		t.Errorf("JSONName(client_secret) = %q, want -", got)
	}
	if got := naming.JSONName("name"); got != "name" {
		t.Errorf("JSONName(name) = %q, want name", got)
	}
	if naming.IsSensitive("user_name") || !naming.IsSensitive("access_token") {
		t.Error("IsSensitive wrong")
	}
}
