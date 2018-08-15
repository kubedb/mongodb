package dormantdatabase

import (
	"github.com/appscode/go/log"
	meta_util "github.com/appscode/kutil/meta"
	"github.com/appscode/kutil/tools/queue"
	"github.com/golang/glog"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Controller) addEventHandler(selector labels.Selector) {
	c.DrmnQueue = queue.New("DormantDatabase", c.MaxNumRequeues, c.NumThreads, c.runDormantDatabase)
	c.DrmnInformer.AddEventHandler(queue.NewFilteredHandler(queue.NewEventHandler(c.DrmnQueue.GetQueue(), func(old interface{}, new interface{}) bool {
		oldObj := old.(*api.DormantDatabase)
		newObj := new.(*api.DormantDatabase)
		if !dormantDatabaseEqual(oldObj, newObj) {
			return true
		}
		return false
	}), selector))
	c.ddbLister = c.KubedbInformerFactory.Kubedb().V1alpha1().DormantDatabases().Lister()
}

func dormantDatabaseEqual(old, new *api.DormantDatabase) bool {
	if api.EnableStatusSubresource {
		if new.Status.ObservedGeneration >= new.Generation {
			return true
		}
		if glog.V(log.LevelDebug) {
			diff := meta_util.Diff(old, new)
			glog.Infof("meta.Generation [%d] is higher than status.observedGeneration [%d] in DormantDatabase %s/%s with Diff: %s",
				new.Generation, new.Status.ObservedGeneration, new.Namespace, new.Name, diff)
		}
		return false
	}
	if !meta_util.Equal(old.Spec, new.Spec) {
		if glog.V(log.LevelDebug) {
			diff := meta_util.Diff(old, new)
			glog.Infof("DormantDatabase %s/%s has changed. Diff: %s", new.Namespace, new.Name, diff)
		}
		return false
	}
	return true
}

func (c *Controller) runDormantDatabase(key string) error {
	log.Debugf("started processing, key: %v", key)
	obj, exists, err := c.DrmnInformer.GetIndexer().GetByKey(key)
	if err != nil {
		log.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}

	if !exists {
		log.Debugf("DormantDatabase %s does not exist anymore", key)
	} else {
		// Note that you also have to check the uid if you have a local controlled resource, which
		// is dependent on the actual instance, to detect that a DormantDatabase was recreated with the same name
		dormantDatabase := obj.(*api.DormantDatabase).DeepCopy()
		if err := c.create(dormantDatabase); err != nil {
			log.Errorln(err)
			return err
		}
	}
	return nil
}
