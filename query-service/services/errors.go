package services

import "errors"

var (
	ErrOptimisticLock    = errors.New("optimistic lock conflict")
	ErrInventoryNotFound = errors.New("inventory not found")
)
