package model

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// InviteCode gates self-registration: a user may only register with a valid, non-expired invite
// code that still has uses left. Mirrors Redemption (admin-generated distinct codes) but is
// MULTI-USE (MaxUses/UsedCount) instead of single-use, and carries no quota. "code" is not a SQL
// reserved word (unlike redemption's "key"), so the consume path needs no per-dialect quoting.
type InviteCode struct {
	Id          int            `json:"id"`
	UserId      int            `json:"user_id"` // the admin who created it
	Code        string         `json:"code" gorm:"type:char(32);uniqueIndex"`
	Status      int            `json:"status" gorm:"default:1"`
	Name        string         `json:"name" gorm:"index"`
	MaxUses     int            `json:"max_uses"` // 0 = unlimited (no gorm default, so 0 persists)
	UsedCount   int            `json:"used_count" gorm:"default:0"`
	CreatedTime int64          `json:"created_time" gorm:"bigint"`
	UsedTime    int64          `json:"used_time" gorm:"bigint"` // last time it was used
	Count       int            `json:"count" gorm:"-:all"`      // only for api request (batch generate)
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	ExpiredTime int64          `json:"expired_time" gorm:"bigint"` // 过期时间，0 表示不过期
}

func GetAllInviteCodes(startIdx int, num int) (codes []*InviteCode, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	err = tx.Model(&InviteCode{}).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	err = tx.Order("id desc").Limit(num).Offset(startIdx).Find(&codes).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return codes, total, nil
}

func SearchInviteCodes(keyword string, startIdx int, num int) (codes []*InviteCode, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&InviteCode{})
	if id, err := strconv.Atoi(keyword); err == nil {
		query = query.Where("id = ? OR name LIKE ?", id, keyword+"%")
	} else {
		query = query.Where("name LIKE ?", keyword+"%")
	}

	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&codes).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return codes, total, nil
}

func GetInviteCodeById(id int) (*InviteCode, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	code := InviteCode{Id: id}
	err := DB.First(&code, "id = ?", id).Error
	return &code, err
}

// ValidateAndConsumeInviteCode atomically verifies a code is usable (enabled, not expired, uses
// left) and burns one use. Row-locked (FOR UPDATE) so concurrent registrations can't over-use a
// code. Returns a descriptive error (shown to the registrant) on any failure; nil on success.
func ValidateAndConsumeInviteCode(code string) error {
	if code == "" {
		return errors.New("请输入邀请码 / invite code required")
	}
	ic := &InviteCode{}
	common.RandomSleep()
	return DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where("code = ?", code).First(ic).Error
		if err != nil {
			return errors.New("邀请码无效 / invalid invite code")
		}
		if ic.Status != common.InviteCodeStatusEnabled {
			return errors.New("邀请码已被禁用 / invite code disabled")
		}
		if ic.ExpiredTime != 0 && ic.ExpiredTime < common.GetTimestamp() {
			return errors.New("邀请码已过期 / invite code expired")
		}
		if ic.MaxUses != 0 && ic.UsedCount >= ic.MaxUses {
			return errors.New("邀请码已用完 / invite code exhausted")
		}
		ic.UsedCount++
		ic.UsedTime = common.GetTimestamp()
		if ic.MaxUses != 0 && ic.UsedCount >= ic.MaxUses {
			ic.Status = common.InviteCodeStatusUsed
		}
		return tx.Save(ic).Error
	})
}

func (ic *InviteCode) Insert() error {
	return DB.Create(ic).Error
}

// Update writes the admin-editable, non-zero fields (mirrors Redemption.Update).
func (ic *InviteCode) Update() error {
	return DB.Model(ic).Select("name", "status", "max_uses", "expired_time").Updates(ic).Error
}

func (ic *InviteCode) Delete() error {
	return DB.Delete(ic).Error
}

func DeleteInviteCodeById(id int) error {
	if id == 0 {
		return errors.New("id 为空！")
	}
	ic := InviteCode{Id: id}
	err := DB.Where(ic).First(&ic).Error
	if err != nil {
		return err
	}
	return ic.Delete()
}

func DeleteInvalidInviteCodes() (int64, error) {
	now := common.GetTimestamp()
	result := DB.Where(
		"status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?)",
		[]int{common.InviteCodeStatusUsed, common.InviteCodeStatusDisabled},
		common.InviteCodeStatusEnabled, now,
	).Delete(&InviteCode{})
	return result.RowsAffected, result.Error
}
