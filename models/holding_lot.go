package models

import (
	"uuid"

	"gorm.io/gorm"
)

// LoadLots returns all lots for a given holding ID.
func LoadLots(db *gorm.DB, holdingID uuid.UUID) ([]HoldingLot, error) {
	var lots []HoldingLot
	if err := db.Where("holding_id = ?", holdingID).Order("date ASC").Find(&lots).Error; err != nil {
		return nil, err
	}
	return lots, nil
}

// LoadLotsByHoldingIDs returns lots grouped by holding ID for multiple holdings.
func LoadLotsByHoldingIDs(db *gorm.DB, holdingIDs []uuid.UUID) (map[uuid.UUID][]HoldingLot, error) {
	if len(holdingIDs) == 0 {
		return make(map[uuid.UUID][]HoldingLot), nil
	}
	var allLots []HoldingLot
	if err := db.Where("holding_id IN ?", holdingIDs).Order("date ASC").Find(&allLots).Error; err != nil {
		return nil, err
	}
	result := make(map[uuid.UUID][]HoldingLot, len(holdingIDs))
	for i := range allLots {
		result[allLots[i].HoldingID] = append(result[allLots[i].HoldingID], allLots[i])
	}
	return result, nil
}

// CreateLot inserts a single lot record.
func CreateLot(db *gorm.DB, lot *HoldingLot) error {
	return db.Create(lot).Error
}

// CreateLots inserts multiple lot records in a batch.
func CreateLots(db *gorm.DB, lots []HoldingLot) error {
	if len(lots) == 0 {
		return nil
	}
	return db.Create(&lots).Error
}

// DeleteLotByID deletes a single lot by its primary key.
func DeleteLotByID(db *gorm.DB, lotID uuid.UUID) error {
	return db.Where("id = ?", lotID).Delete(&HoldingLot{}).Error
}

// DeleteLotsByHoldingID deletes all lots for a given holding.
func DeleteLotsByHoldingID(db *gorm.DB, holdingID uuid.UUID) error {
	return db.Where("holding_id = ?", holdingID).Delete(&HoldingLot{}).Error
}

// ReplaceLots atomically deletes all existing lots for a holding and creates new ones.
func ReplaceLots(db *gorm.DB, holdingID uuid.UUID, newLots []HoldingLot) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("holding_id = ?", holdingID).Delete(&HoldingLot{}).Error; err != nil {
			return err
		}
		if len(newLots) > 0 {
			for i := range newLots {
				newLots[i].HoldingID = holdingID
			}
			if err := tx.Create(&newLots).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
