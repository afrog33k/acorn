package publish

import (
	"errors"
	"testing"

	v1 "github.com/acorn-io/acorn/pkg/apis/internal.acorn.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestToEndpoint(t *testing.T) {
	type args struct {
		domain      string
		serviceName string
		pattern     string
		appInstance *v1.AppInstance
	}
	tests := []struct {
		name string
		args args

		wantEndpoint string
		wantErr      error
	}{
		{
			name: "\"on-acorn.io no -\" pattern set",
			args: args{
				domain:      "domain.on-acorn.io",
				serviceName: "app-test",
				pattern:     "{{.Container}}-{{.App}}-{{.Hash}}.{{.ClusterDomain}}",
				appInstance: &v1.AppInstance{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{Name: "green-star"},
					Spec:       v1.AppInstanceSpec{},
					Status:     v1.AppInstanceStatus{},
				},
			},
			wantEndpoint: "app-test-green-star-b19d0b346674.domain.on-acorn.io",
		},
		{
			name: "\"custom domain\" pattern set",
			args: args{
				domain:      "domain.custom-domain.io",
				serviceName: "app-test",
				pattern:     "{{.Container}}.{{.App}}.{{.Namespace}}.{{.ClusterDomain}}",
				appInstance: &v1.AppInstance{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{Name: "green-star", Namespace: "namespace"},
					Spec:       v1.AppInstanceSpec{},
					Status:     v1.AppInstanceStatus{},
				},
			},
			wantEndpoint: "app-test.green-star.namespace.domain.custom-domain.io",
		},
		{
			name: "\"custom domain default service\" pattern set",
			args: args{
				domain:      "domain.custom-domain.io",
				serviceName: "default",
				pattern:     "{{.App}}.{{.Namespace}}.{{.ClusterDomain}}",
				appInstance: &v1.AppInstance{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{Name: "green-star", Namespace: "namespace"},
					Spec:       v1.AppInstanceSpec{},
					Status:     v1.AppInstanceStatus{},
				},
			},
			wantEndpoint: "green-star.namespace.domain.custom-domain.io",
		},
		{
			name: "friendly pattern set",
			args: args{
				domain:      "custom-domain.io",
				serviceName: "app-test",
				pattern:     "{{.Container}}.{{.App}}.{{.Namespace}}.{{.ClusterDomain}}",
				appInstance: &v1.AppInstance{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{Name: "green-star", Namespace: "namespace"},
					Spec:       v1.AppInstanceSpec{},
					Status:     v1.AppInstanceStatus{},
				},
			},
			wantEndpoint: "app-test.green-star.namespace.custom-domain.io",
		},
		{
			name: "lets encrypt pattern set",
			args: args{
				domain:      "custom-domain.io",
				serviceName: "app-test",
				pattern:     "{{.Container}}-{{.App}}-{{.Hash}}.{{.ClusterDomain}}",
				appInstance: &v1.AppInstance{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{Name: "green-star", Namespace: "namespace"},
					Spec:       v1.AppInstanceSpec{},
					Status:     v1.AppInstanceStatus{},
				},
			},
			wantEndpoint: "app-test-green-star-49eba2c97fa7.custom-domain.io",
		},
		{
			name: "custom pattern set",
			args: args{
				domain:      "custom-domain.io",
				serviceName: "app-test",
				pattern:     "{{.Container}}-{{.App}}.{{.Namespace}}-cluster.{{.ClusterDomain}}",
				appInstance: &v1.AppInstance{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{Name: "green-star", Namespace: "namespace"},
					Spec:       v1.AppInstanceSpec{},
					Status:     v1.AppInstanceStatus{},
				},
			},
			wantEndpoint: "app-test-green-star.namespace-cluster.custom-domain.io",
		},
		{
			name: "no pattern set",
			args: args{
				domain:      "custom-domain.io",
				serviceName: "app-test",
				pattern:     "",
				appInstance: &v1.AppInstance{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{Name: "green-star", Namespace: "namespace"},
					Spec:       v1.AppInstanceSpec{},
					Status:     v1.AppInstanceStatus{},
				},
			},
			wantEndpoint: "app-test-green-star-49eba2c97fa7.custom-domain.io",
		},
		{
			name: "bad pattern set",
			args: args{
				domain:      "custom-domain.io",
				serviceName: "app-test",
				pattern:     "{{.Foo}}-{{.Bar}}.{{.Baz}}-cluster.{{.Bat}}",
				appInstance: &v1.AppInstance{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{Name: "green-star", Namespace: "namespace"},
					Spec:       v1.AppInstanceSpec{},
					Status:     v1.AppInstanceStatus{},
				},
			},
			wantEndpoint: "",
			wantErr:      ErrPatternParseFailed,
		},
		{
			name: "parsed pattern's segment exceeds maximum length",
			args: args{
				domain:      "custom-domain.io",
				serviceName: "app-name-that-is-very-long-and-should-cause-issues",
				pattern:     "{{.Container}}-{{.App}}-{{.Hash}}-{{.Namespace}}.{{.ClusterDomain}}",
				appInstance: &v1.AppInstance{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{Name: "green-star", Namespace: "namespace"},
					Spec:       v1.AppInstanceSpec{},
					Status:     v1.AppInstanceStatus{},
				},
			},
			wantEndpoint: "",
			wantErr:      ErrSegmentExceededMaxLength,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEndpoint, err := toEndpoint(tt.args.pattern, tt.args.domain, tt.args.serviceName, tt.args.appInstance.GetName(), tt.args.appInstance.GetNamespace())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("toEndpoint() error = %v, want %v", err, tt.wantErr)
			}

			if gotEndpoint != tt.wantEndpoint {
				t.Errorf("toEndpoint() = %v, want %v", gotEndpoint, tt.wantEndpoint)
			}
		})
	}
}
