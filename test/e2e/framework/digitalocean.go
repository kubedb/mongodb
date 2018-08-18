package framework

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CloudProviderEnvKey  = "ClusterProvider"
	DigitalOceanCloudEnv = "digitalocean"
)

func (f *Framework) GetDORateLimitRemaining() (int, error) {
	secret, err := f.kubeClient.CoreV1().Secrets("kube-system").Get("digitalocean", metav1.GetOptions{})
	if err != nil {
		return 0, err
	}

	token := string(secret.Data["token"])
	var bearer = "Bearer " + token

	client := &http.Client{}
	req, _ := http.NewRequest("GET", "https://api.digitalocean.com/v2/", nil)
	req.Header.Set("authorization", bearer)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := client.Do(req)

	defer resp.Body.Close()

	limitRemainString := resp.Header.Get("ratelimit-remaining")
	limitRemain, err := strconv.Atoi(limitRemainString)

	return limitRemain, err
}

// If cloudProvider is digitalocean, check for remaining API request amount.
// ref: https://developers.digitalocean.com/documentation/v2/#rate-limit
func (f *Framework) WaitUntilDigitalOceanReady() {
	if f.CloudProvider == DigitalOceanCloudEnv {
		By("Checking available DigitalOcean API rate limit remaining")
		// wait maximum 20 minutes for APILimits to be available at least 500
		for i := 0; i < 20; i++ {
			limitRemaining, err := f.GetDORateLimitRemaining()
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
			fmt.Println("DigitalOcean API ratelimit-remaining:", limitRemaining)
			if limitRemaining >= 500 {
				return
			}
			time.Sleep(time.Second * 60)
		}
	}
}
