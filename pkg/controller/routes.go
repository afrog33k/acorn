package controller

import (
	"net/http"

	v1 "github.com/acorn-io/acorn/pkg/apis/internal.acorn.io/v1"
	"github.com/acorn-io/acorn/pkg/controller/appdefinition"
	"github.com/acorn-io/acorn/pkg/controller/builder"
	"github.com/acorn-io/acorn/pkg/controller/config"
	"github.com/acorn-io/acorn/pkg/controller/gc"
	"github.com/acorn-io/acorn/pkg/controller/ingress"
	"github.com/acorn-io/acorn/pkg/controller/namespace"
	"github.com/acorn-io/acorn/pkg/controller/pvc"
	"github.com/acorn-io/acorn/pkg/controller/tls"
	"github.com/acorn-io/acorn/pkg/labels"
	"github.com/acorn-io/acorn/pkg/system"
	"github.com/acorn-io/baaah/pkg/router"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
)

var (
	managedSelector = klabels.SelectorFromSet(map[string]string{
		labels.AcornManaged: "true",
	})
)

func routes(router *router.Router, registryTransport http.RoundTripper) {
	router.OnErrorHandler = appdefinition.OnError

	router.HandleFunc(&v1.AppInstance{}, appdefinition.AssignNamespace)
	router.HandleFunc(&v1.AppInstance{}, appdefinition.PullAppImage(registryTransport))
	router.HandleFunc(&v1.AppInstance{}, appdefinition.ParseAppImage)
	router.HandleFunc(&v1.AppInstance{}, tls.ProvisionCerts) // Provision TLS certificates for port bindings with user-defined (valid) domains

	// DeploySpec will create the namespace, so ensure it runs before anything that requires a namespace
	appRouter := router.Type(&v1.AppInstance{}).Middleware(appdefinition.RequireNamespace).Middleware(appdefinition.IgnoreTerminatingNamespace)
	appRouter.Middleware(appdefinition.ImagePulled).Middleware(appdefinition.CheckDependencies).HandlerFunc(appdefinition.DeploySpec)
	appRouter.Middleware(appdefinition.ImagePulled).HandlerFunc(appdefinition.CreateSecrets)
	appRouter.HandlerFunc(appdefinition.AppStatus)
	appRouter.HandlerFunc(appdefinition.AppEndpointsStatus)
	appRouter.HandlerFunc(appdefinition.JobStatus)
	appRouter.HandlerFunc(appdefinition.ReadyStatus)
	appRouter.HandlerFunc(appdefinition.CLIStatus)
	appRouter.HandlerFunc(appdefinition.UpdateGeneration)

	router.Type(&v1.BuilderInstance{}).HandlerFunc(builder.DeployBuilder)

	router.Type(&rbacv1.ClusterRole{}).Selector(managedSelector).HandlerFunc(gc.GCOrphans)
	router.Type(&rbacv1.ClusterRoleBinding{}).Selector(managedSelector).HandlerFunc(gc.GCOrphans)
	router.Type(&corev1.PersistentVolumeClaim{}).Selector(managedSelector).HandlerFunc(pvc.MarkAndSave)
	router.Type(&corev1.PersistentVolume{}).Selector(managedSelector).HandlerFunc(appdefinition.ReleaseVolume)
	router.Type(&corev1.Namespace{}).Selector(managedSelector).HandlerFunc(namespace.DeleteOrphaned)
	router.Type(&appsv1.DaemonSet{}).Namespace(system.Namespace).HandlerFunc(gc.GCOrphans)
	router.Type(&appsv1.Deployment{}).Namespace(system.Namespace).HandlerFunc(gc.GCOrphans)
	router.Type(&corev1.Service{}).Namespace(system.Namespace).HandlerFunc(gc.GCOrphans)
	router.Type(&corev1.Pod{}).Selector(managedSelector).HandlerFunc(gc.GCOrphans)
	router.Type(&netv1.Ingress{}).Selector(managedSelector).Middleware(ingress.RequireLBs).Handler(ingress.NewDNSHandler())
	router.Type(&corev1.ConfigMap{}).Namespace(system.Namespace).Name(system.ConfigName).Handler(config.NewDNSConfigHandler())
	router.Type(&corev1.ConfigMap{}).Namespace(system.Namespace).Name(system.ConfigName).HandlerFunc(builder.DeployRegistry)
	router.Type(&corev1.Secret{}).Selector(managedSelector).Middleware(tls.RequireSecretTypeTLS).HandlerFunc(tls.RenewCert) // renew (expired) TLS certificates, including the on-acorn.io wildcard cert
	router.Type(&corev1.ConfigMap{}).Namespace(system.Namespace).Name(system.ConfigName).HandlerFunc(config.HandleAutoUpgradeInterval)
}
