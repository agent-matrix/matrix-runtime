// Copyright The MatrixHub Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

func IsFullPageData(page, pageSize int) bool {
	return page == 1 && pageSize == -1
}

// CalculatePages calculates the total number of pages based on total count and page size.
// It returns 0 if pageSize is 0 or negative.
func CalculatePages(total int64, pageSize int32) int32 {
	if pageSize <= 0 {
		return 0
	}
	return (int32(total) + pageSize - 1) / pageSize
}
