package utils

import "strconv"

// ParsePage 解析分页参数，默认 page=1, pageSize=10
func ParsePage(pageStr, pageSizeStr string) (int, int) {
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}
