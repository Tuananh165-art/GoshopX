package utils

import "github.com/rasadov/EcommerceAPI/graphql/generated"

func Bounds(pagination *generated.PaginationInput) (uint64, uint64) {
	skipValue := uint64(0)
	takeValue := uint64(100)
	if pagination.Skip != 0 {
		skipValue = uint64(pagination.Skip)
	}
	if pagination.Take != 100 {
		takeValue = uint64(pagination.Take)
	}
	return skipValue, takeValue
}
