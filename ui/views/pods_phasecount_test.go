package views

import (
	"testing"

	"github.com/hermanu/klens/k8s/resources"
)

// TestPodsView_PhaseCounts verifies the phase aggregation buckets every
// pod into Running / Pending / Errored / Total per the rules documented
// on PhaseCounts. Total counts every pod regardless of bucket.
func TestPodsView_PhaseCounts(t *testing.T) {
	cases := []struct {
		name                           string
		statuses                       []string
		wantR, wantP, wantE, wantTotal int
	}{
		{
			name:      "all-running",
			statuses:  []string{"Running", "Running", "Running"},
			wantR:     3,
			wantTotal: 3,
		},
		{
			name:      "pending bucket includes transitional reasons",
			statuses:  []string{"Pending", "ContainerCreating", "PodInitializing"},
			wantP:     3,
			wantTotal: 3,
		},
		{
			name:      "error bucket includes Failed/Error/CrashLoop/ImagePull/Evicted/OOMKilled",
			statuses:  []string{"Failed", "Error", "CrashLoopBackOff", "ImagePullBackOff", "Evicted", "OOMKilled"},
			wantE:     6,
			wantTotal: 6,
		},
		{
			name:      "succeeded/completed/terminating count only toward total",
			statuses:  []string{"Succeeded", "Completed", "Terminating"},
			wantTotal: 3,
		},
		{
			name:      "mixed",
			statuses:  []string{"Running", "Running", "Pending", "CrashLoopBackOff", "Completed", "Unknown"},
			wantR:     2,
			wantP:     1,
			wantE:     1,
			wantTotal: 6,
		},
		{
			name:      "empty list",
			statuses:  nil,
			wantTotal: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pods := make([]resources.PodItem, len(tc.statuses))
			for i, s := range tc.statuses {
				pods[i] = resources.PodItem{Status: s}
			}
			v := PodsView{pods: pods}
			r, p, e, total := v.PhaseCounts()
			if r != tc.wantR || p != tc.wantP || e != tc.wantE || total != tc.wantTotal {
				t.Errorf("counts = (R=%d P=%d E=%d T=%d), want (R=%d P=%d E=%d T=%d)",
					r, p, e, total, tc.wantR, tc.wantP, tc.wantE, tc.wantTotal)
			}
		})
	}
}
