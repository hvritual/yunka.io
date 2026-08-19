package orm

import "gorm.io/gorm"

// TransactionDB returns the currently active request transaction. It returns
// false when Begin has not succeeded, preventing callers from accidentally
// treating the base ORM handle as a transactional-outbox handle.
func (orm *ORM) TransactionDB() (*gorm.DB, bool) {
	if orm == nil || !orm.begin || orm.DB == nil {
		return nil, false
	}
	return orm.DB, true
}
