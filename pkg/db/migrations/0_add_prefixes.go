package migrations

import (
	"encoding/json"

	"github.com/lueurxax/teaelephantmemory/common"
)

type addPrefixes struct {
}

func (a *addPrefixes) Migrate(db MigratingDB) error {
	records, err := db.ReadAll()
	if err != nil {
		return err
	}
	for _, record := range records {
		rec := new(common.Record)
		if err = json.Unmarshal(record.Value, rec); err != nil {
			return err
		}
		if _, err = db.Update(string(record.Key), rec); err != nil {
			return err
		}
	}
	return nil
}
