package recordsets

import (
	"encoding/json"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/pagination"
)

// RecordSet represents a DNS Record Set (copied from gophercloud implementation).
type RecordSet struct {
	ID          string             `json:"id"`
	ZoneID      string             `json:"zone_id"`
	ProjectID   string             `json:"project_id"`
	Name        string             `json:"name"`
	ZoneName    string             `json:"zone_name"`
	Type        string             `json:"type"`
	Records     []string           `json:"records"`
	TTL         int                `json:"ttl"`
	Status      string             `json:"status"`
	Action      string             `json:"action"`
	Description string             `json:"description"`
	Version     int                `json:"version"`
	CreatedAt   time.Time          `json:"-"`
	UpdatedAt   time.Time          `json:"-"`
	Links       []gophercloud.Link `json:"-"`
	Metadata    struct {
		TotalCount int `json:"total_count"`
	} `json:"metadata"`
}

func (r *RecordSet) UnmarshalJSON(b []byte) error {
	type recordsetAlias RecordSet
	var s struct {
		recordsetAlias
		CreatedAt gophercloud.JSONRFC3339MilliNoZ `json:"created_at"`
		UpdatedAt gophercloud.JSONRFC3339MilliNoZ `json:"updated_at"`
		Links     map[string]interface{}          `json:"links"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*r = RecordSet(s.recordsetAlias)

	r.CreatedAt = time.Time(s.CreatedAt)
	r.UpdatedAt = time.Time(s.UpdatedAt)

	if s.Links != nil {
		for rel, href := range s.Links {
			if v, ok := href.(string); ok {
				link := gophercloud.Link{Rel: rel, Href: v}
				r.Links = append(r.Links, link)
			}
		}
	}
	return nil
}

// RecordSetPage is a single page of RecordSet results.
type RecordSetPage struct {
	pagination.LinkedPageBase
}

// IsEmpty checks whether a page of results has any RecordSets.
func (r RecordSetPage) IsEmpty() (bool, error) {
	if r.StatusCode == 204 {
		return true, nil
	}
	sets, err := ExtractRecordSets(r)
	return len(sets) == 0, err
}

// ExtractRecordSets extracts a slice of RecordSets from a Page.
func ExtractRecordSets(p pagination.Page) ([]RecordSet, error) {
	var s struct {
		RecordSets []RecordSet `json:"recordsets"`
	}
	err := (p.(RecordSetPage)).ExtractInto(&s)
	return s.RecordSets, err
}
