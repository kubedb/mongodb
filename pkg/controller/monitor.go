package controller

import (
	"fmt"

	tapi "github.com/k8sdb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/k8sdb/apimachinery/pkg/monitor"
)

func (c *Controller) newMonitorController(mongodb *tapi.MongoDB) (monitor.Monitor, error) {
	monitorSpec := mongodb.Spec.Monitor

	if monitorSpec == nil {
		return nil, fmt.Errorf("MonitorSpec not found in %v", mongodb.Spec)
	}

	if monitorSpec.Prometheus != nil {
		return monitor.NewPrometheusController(c.Client, c.ApiExtKubeClient, c.promClient, c.opt.OperatorNamespace), nil
	}

	return nil, fmt.Errorf("Monitoring controller not found for %v", monitorSpec)
}

func (c *Controller) addMonitor(mongodb *tapi.MongoDB) error {
	ctrl, err := c.newMonitorController(mongodb)
	if err != nil {
		return err
	}
	return ctrl.AddMonitor(mongodb.ObjectMeta, mongodb.Spec.Monitor)
}

func (c *Controller) deleteMonitor(mongodb *tapi.MongoDB) error {
	ctrl, err := c.newMonitorController(mongodb)
	if err != nil {
		return err
	}
	return ctrl.DeleteMonitor(mongodb.ObjectMeta, mongodb.Spec.Monitor)
}

func (c *Controller) updateMonitor(oldMongoDB, updatedMongoDB *tapi.MongoDB) error {
	var err error
	var ctrl monitor.Monitor
	if updatedMongoDB.Spec.Monitor == nil {
		ctrl, err = c.newMonitorController(oldMongoDB)
	} else {
		ctrl, err = c.newMonitorController(updatedMongoDB)
	}
	if err != nil {
		return err
	}
	return ctrl.UpdateMonitor(updatedMongoDB.ObjectMeta, oldMongoDB.Spec.Monitor, updatedMongoDB.Spec.Monitor)
}
