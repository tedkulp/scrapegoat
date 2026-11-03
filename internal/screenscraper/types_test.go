package screenscraper

import (
	"encoding/json"
	"reflect"
	"testing"
)

// ssuserInfosFixture uses the real ssuser key set returned by ssuserInfos.php
// (checked 2026-11-02), with fake values. ScreenScraper sends values as strings.
const ssuserInfosFixture = `{
	"response": {
		"ssuser": {
			"id": "someone",
			"numid": "12345",
			"niveau": "1",
			"contribution": "0",
			"uploadsysteme": "0",
			"uploadinfos": "0",
			"romasso": "0",
			"uploadmedia": "0",
			"propositionok": "0",
			"propositionko": "0",
			"quotarefu": "0",
			"maxthreads": "1",
			"maxdownloadspeed": "128",
			"requeststoday": "42",
			"requestskotoday": "7",
			"maxrequestspermin": "3072",
			"maxrequestsperday": "20000",
			"maxrequestskoperday": "2000",
			"visites": "5",
			"datedernierevisite": "2026-11-02 12:00:00",
			"favregion": "us"
		}
	}
}`

func TestUserInfoDecodesQuotaFieldsFromSSUserInfos(t *testing.T) {
	var resp UserInfoJSONResponse
	if err := json.Unmarshal([]byte(ssuserInfosFixture), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	u := resp.Response.SSUser

	checks := []struct{ name, got, want string }{
		{"NumeroID", u.NumeroID, "12345"},
		{"MaxRequestsPerMinute", u.MaxRequestsPerMinute, "3072"},
		{"MaxRequestsPerDay", u.MaxRequestsPerDay, "20000"},
		{"RequestsToday", u.RequestsToday, "42"},
		{"RequestsKOToday", u.RequestsKOToday, "7"},
		{"MaxRequestsKOPerDay", u.MaxRequestsKOPerDay, "2000"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestDefaultRegionsOrder(t *testing.T) {
	want := []string{"us", "wor", "eu", "jp"}
	if got := DefaultRegions(); !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultRegions() = %v, want %v", got, want)
	}
}

func TestDefaultRegionsReturnsFreshSlice(t *testing.T) {
	first := DefaultRegions()
	first[0] = "changed"
	if got := DefaultRegions()[0]; got != "us" {
		t.Fatalf("mutating one result changed the default: first region = %q", got)
	}
}
