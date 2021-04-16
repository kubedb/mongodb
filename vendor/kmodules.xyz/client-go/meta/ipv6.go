package meta

import (
	"context"
	"io/ioutil"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func IPv6EnabledOnCluster(kc kubernetes.Interface) (bool, error) {
	svc, err := kc.CoreV1().Services(metav1.NamespaceDefault).Get(context.TODO(), "kubernetes", metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	clusterIPs := []string{svc.Spec.ClusterIP}
	for _, ip := range clusterIPs {
		if strings.ContainsRune(ip, ':') {
			return true, nil
		}
	}
	return false, nil
}

func IPv6EnabledOnKernel() (bool, error) {
	content, err := ioutil.ReadFile("/sys/module/ipv6/parameters/disable")
	if err != nil {
		return false, err
	}
	if strings.Contains(string(content), "0" ) {
		return true, err
	}
	return false, nil
}