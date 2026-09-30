package app

import (
	"strings"
	"testing"
	"time"
)

func validOptions() Options {
	return Options{
		ConfigFile:              "/etc/topolvm/capacity-template.yaml",
		LeaderElect:             true,
		LeaderElectionNamespace: "kube-system",
		LeaderElectionID:        "datadog-topolvm-capacity-template-controller",
		MetricsBindAddress:      ":8082",
		HealthProbeBindAddress:  ":8083",
		SyncPeriod:              5 * time.Minute,
	}
}

func TestValidateOptions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Options)
		want   string
	}{
		{name: "config", mutate: func(o *Options) { o.ConfigFile = "" }, want: "config"},
		{
			name:   "leader namespace",
			mutate: func(o *Options) { o.LeaderElectionNamespace = "Bad_Value" },
			want:   "leader-election-namespace",
		},
		{name: "leader id", mutate: func(o *Options) { o.LeaderElectionID = "Bad_Value" }, want: "leader-election-id"},
		{name: "metrics address", mutate: func(o *Options) { o.MetricsBindAddress = "8082" }, want: "metrics-bind-address"},
		{
			name:   "health address",
			mutate: func(o *Options) { o.HealthProbeBindAddress = "" },
			want:   "health-probe-bind-address",
		},
		{name: "sync period", mutate: func(o *Options) { o.SyncPeriod = 0 }, want: "sync-period"},
	}

	if err := ValidateOptions(validOptions()); err != nil {
		t.Fatalf("valid options rejected: %v", err)
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := validOptions()
			tc.mutate(&opts)
			err := ValidateOptions(opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateOptions() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestCommandRejectsInvalidFlagsBeforeStartup(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"--config="})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "config") {
		t.Fatalf("Execute() error = %v", err)
	}
}
