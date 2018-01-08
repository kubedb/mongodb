package controller

import (
	"fmt"

	"github.com/appscode/kube-mon/agents"
	mona "github.com/appscode/kube-mon/api"
	"github.com/appscode/kutil"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/appscode/go/log"
)

func (c *Controller) newMonitorController(mongodb *api.MongoDB) (mona.Agent, error) {
	monitorSpec := mongodb.Spec.Monitor

	if monitorSpec == nil {
		return nil, fmt.Errorf("MonitorSpec not found in %v", mongodb.Spec)
	}

	if monitorSpec.Prometheus != nil {
		return agents.New(monitorSpec.Agent, c.Client, c.ApiExtKubeClient, c.promClient), nil
	}

	return nil, fmt.Errorf("monitoring controller not found for %v", monitorSpec)
}

func (c *Controller) addOrUpdateMonitor(mongodb *api.MongoDB) (kutil.VerbType, error) {
	agent, err := c.newMonitorController(mongodb)
	if err != nil {
		return kutil.VerbUnchanged, err
	}
	return agent.CreateOrUpdate(mongodb.StatsAccessor(), mongodb.Spec.Monitor)
}

func (c *Controller) deleteMonitor(mongodb *api.MongoDB) (kutil.VerbType, error) {
	agent, err := c.newMonitorController(mongodb)
	if err != nil {
		return kutil.VerbUnchanged, err
	}
	return agent.Delete(mongodb.StatsAccessor())
}

func (c *Controller) manageMonitor(mongodb *api.MongoDB) error {
	if mongodb.Spec.Monitor != nil {
		_, err := c.addOrUpdateMonitor(mongodb)
		return err
	}
	agent := agents.New(mona.AgentCoreOSPrometheus, c.Client, c.ApiExtKubeClient, c.promClient)
	if _, err := agent.Delete(mongodb.StatsAccessor()); err != nil {
		log.Debugf("error in deleting Prometheus agent:", err)
	}
	agent = agents.New(mona.AgentPrometheusBuiltin, c.Client, c.ApiExtKubeClient, c.promClient)
	if _, err := agent.Delete(mongodb.StatsAccessor()); err != nil {
		log.Debugf("error in deleting Prometheus agent:", err)
	}
	return nil
}
