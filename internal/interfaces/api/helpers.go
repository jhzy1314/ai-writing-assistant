package api

import "time"

const reqTimeout = 3 * time.Minute // 单次非流式 LLM 请求超时
