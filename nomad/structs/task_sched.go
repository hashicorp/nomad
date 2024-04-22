// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

type TaskSchedule struct {
	Cron *TaskScheduleCron
}

type TaskScheduleCron struct {
	Start    string
	Stop     string
	Timezone string
}
