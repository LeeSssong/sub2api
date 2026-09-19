package service

import "time"

func unifiedQualityTestAccount(id, groupID int64) Account {
	rate := 0.1
	return Account{ID: id, Name: "test", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, RateMultiplier: &rate, GroupIDs: []int64{groupID}, CreatedAt: time.Now()}
}
