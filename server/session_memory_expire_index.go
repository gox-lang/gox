package server

import "time"

type expireIndex struct {
	buckets       map[int64]map[string]struct{}
	sessionBucket map[string]int64
	cursor        int64
}

func newExpireIndex() *expireIndex {
	nowBucket := time.Now().Unix() / 60

	return &expireIndex{
		buckets:       make(map[int64]map[string]struct{}),
		sessionBucket: make(map[string]int64),
		cursor:        nowBucket,
	}
}

func (idx *expireIndex) Upsert(sessionID string, expireAt time.Time) bool {
	if idx == nil || sessionID == "" || expireAt.IsZero() {
		return false
	}

	newBucket := expireBucket(expireAt)

	if oldBucket, ok := idx.sessionBucket[sessionID]; ok {
		if oldBucket == newBucket {
			return false
		}

		idx.removeFromBucket(sessionID, oldBucket)
	}

	bucket := idx.buckets[newBucket]
	if bucket == nil {
		bucket = make(map[string]struct{})
		idx.buckets[newBucket] = bucket
	}

	bucket[sessionID] = struct{}{}
	idx.sessionBucket[sessionID] = newBucket

	return true
}

func (idx *expireIndex) Remove(sessionID string) bool {
	if idx == nil || sessionID == "" {
		return false
	}

	bucket, ok := idx.sessionBucket[sessionID]
	if !ok {
		return false
	}

	idx.removeFromBucket(sessionID, bucket)
	delete(idx.sessionBucket, sessionID)

	return true
}

func (idx *expireIndex) PopDue(now time.Time) (string, bool) {
	if idx == nil {
		return "", false
	}

	nowBucket := now.Unix() / 60
	if idx.cursor > nowBucket {
		return "", false
	}

	for idx.cursor <= nowBucket {
		bucket := idx.buckets[idx.cursor]
		if len(bucket) == 0 {
			delete(idx.buckets, idx.cursor)
			idx.cursor++
			continue
		}

		for sessionID := range bucket {
			delete(bucket, sessionID)
			delete(idx.sessionBucket, sessionID)

			if len(bucket) == 0 {
				delete(idx.buckets, idx.cursor)
				idx.cursor++
			}

			return sessionID, true
		}
	}

	return "", false
}

func (idx *expireIndex) removeFromBucket(sessionID string, bucketMinute int64) {
	bucket := idx.buckets[bucketMinute]
	if bucket == nil {
		return
	}

	delete(bucket, sessionID)
	if len(bucket) == 0 {
		delete(idx.buckets, bucketMinute)
	}
}

func expireBucket(expireAt time.Time) int64 {
	unix := expireAt.Unix()
	bucket := unix / 60
	if unix%60 != 0 || expireAt.Nanosecond() != 0 {
		bucket++
	}

	return bucket
}

func nextMinuteDelay(now time.Time) time.Duration {
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	return time.Until(nextMinute)
}
