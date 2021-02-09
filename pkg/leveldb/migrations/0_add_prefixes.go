package migrations

import (
	"context"
	"encoding/json"

	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

type addPrefixes struct {
}

func (a *addPrefixes) Migrate(db MigratingDB) error {
	records, err := db.ReadAll(context.TODO())
	if err != nil {
		return err
	}
	for _, record := range records {
		rec := new(common.TeaData)
		if err = json.Unmarshal(record.Value, rec); err != nil {
			return err
		}
		if _, err = db.Update(context.TODO(), uuid.FromBytesOrNil(record.Key), rec); err != nil {
			return err
		}
	}
	return nil
}
