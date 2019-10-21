/*
Copyright 2019 Baidu, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8sclient

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned/fake"
)

const (
	kubeConfigBody = `
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUR3akNDQXFxZ0F3SUJBZ0lVUkxLSXpQalhMVGl1MWQvcVFkMEgrRWFqOW5Nd0RRWUpLb1pJaHZjTkFRRUwKQlFBd1p6RUxNQWtHQTFVRUJoTUNRMDR4RVRBUEJnTlZCQWdUQ0ZOb1pXNTZhR1Z1TVJFd0R3WURWUVFIRXdoVAphR1Z1ZW1obGJqRU1NQW9HQTFVRUNoTURhemh6TVE4d0RRWURWUVFMRXdaVGVYTjBaVzB4RXpBUkJnTlZCQU1UCkNtdDFZbVZ5Ym1WMFpYTXdIaGNOTVRrd016SXdNVEl3TkRBd1doY05Namt3TXpFM01USXdOREF3V2pCbk1Rc3cKQ1FZRFZRUUdFd0pEVGpFUk1BOEdBMVVFQ0JNSVUyaGxibnBvWlc0eEVUQVBCZ05WQkFjVENGTm9aVzU2YUdWdQpNUXd3Q2dZRFZRUUtFd05yT0hNeER6QU5CZ05WQkFzVEJsTjVjM1JsYlRFVE1CRUdBMVVFQXhNS2EzVmlaWEp1ClpYUmxjekNDQVNJd0RRWUpLb1pJaHZjTkFRRUJCUUFEZ2dFUEFEQ0NBUW9DZ2dFQkFMbDB2cytrTkR2dHNaREUKVk1acnJ4N09XblA4OTNhWCtFaW5PaytVTmUzRDVtUVUyTWhSOGpjTjl6SlZRSk1oQ3NZY2Fld2hOOG9mY3hGbwo1RG9pUFFweDF5UHVUSjdOZmRiK2wvVW5yYTQ4aEY1WXcvUVBUdmdDd0xyVWRQdkxhVWNlSnJsMjFjYjBEMVVWCm1JL3M2ZlJNM0c3Y0VlZVRvdlZvcVVRU1RhL3NkL1QwMVZoQWZPREk2ckFnbmJRQW15SWhYZ2YwZ2ZmY292dm4KUjUwR2NXMEtUMzdzRWduMGVDTWNGT1laM0xOamJJVU1tQU82cWk0U1RMb0lCMXFKWkV0UWd2NEZQZFNLK09pdQpmQndMNFNyeXBDZy9wWkJyVEVWbkJmVndmejZ5aHFRKytBV2I2am0wS2IzWFo0cDhidFRpNHRzVGVoZWduVkZkCjMvL00vTUVDQXdFQUFhTm1NR1F3RGdZRFZSMFBBUUgvQkFRREFnRUdNQklHQTFVZEV3RUIvd1FJTUFZQkFmOEMKQVFJd0hRWURWUjBPQkJZRUZOeFBLS0c2MXF5cXZCUTRwVGpOdFBHeFNHVUVNQjhHQTFVZEl3UVlNQmFBRk54UApLS0c2MXF5cXZCUTRwVGpOdFBHeFNHVUVNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUFzTE4zY0tWQklvTVJNCkdRWGxzaUJLeUpFM1JET0k0ZVhwbzlXSnVyMlNLbjdJU0NWMTNEQlZQOFptM1FVR0l0cXhBaTQyQVdUdmNzdFYKUlRtYXBQQlYrUm1NSHAzRnRGMWt5NkFNa2VRV3NvSWFKZ2xERkVYZElFaWZ0Q0JNbzUzNVVWbTdTRmNObmpDdQpSK0liL08yZHIyVXZOYnJ3YkNHN2o5bDhzSTcxMUVPVWcwNk9IZFp4eEVjU1VFQTVrODdkMjdDaDNjUmZaazhHCm9jSThsWFhLcDFHVUJPbzduVkFNeVMyZCsramQ4ZG9pd3B3cHpkNkhCb0tDYTF2dE5OT3hyZFY0OGVEanE1YzIKVHY0T284aG9RN2c5bkV5MlNDM0p2TDB5bS9UZGR1VWIzK1RRQWJ4eFVzOS9BTFNSeUxEcEVGNFBsVlNNb0hKVwo4VUQ1eHVuOAotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0aCg==
    server: http://10.162.35.7:880
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: admin
  name: kubernetes
current-context: kubernetes
kind: Config
preferences: {}
users:
- name: admin
  user:
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUQ0VENDQXNtZ0F3SUJBZ0lVUnJVM1hRQ0t0dlFpSGhzRVcyckZ2SURiMEhJd0RRWUpLb1pJaHZjTkFRRUwKQlFBd1p6RUxNQWtHQTFVRUJoTUNRMDR4RVRBUEJnTlZCQWdUQ0ZOb1pXNTZhR1Z1TVJFd0R3WURWUVFIRXdoVAphR1Z1ZW1obGJqRU1NQW9HQTFVRUNoTURhemh6TVE4d0RRWURWUVFMRXdaVGVYTjBaVzB4RXpBUkJnTlZCQU1UCkNtdDFZbVZ5Ym1WMFpYTXdIaGNOTVRrd016SXdNVEl3TkRBd1doY05Namt3TXpFM01USXdOREF3V2pCdE1Rc3cKQ1FZRFZRUUdFd0pEVGpFUk1BOEdBMVVFQ0JNSVUyaGxibnBvWlc0eEVUQVBCZ05WQkFjVENGTm9aVzU2YUdWdQpNUmN3RlFZRFZRUUtFdzV6ZVhOMFpXMDZiV0Z6ZEdWeWN6RVBNQTBHQTFVRUN4TUdVM2x6ZEdWdE1RNHdEQVlEClZRUURFd1ZoWkcxcGJqQ0NBU0l3RFFZSktvWklodmNOQVFFQkJRQURnZ0VQQURDQ0FRb0NnZ0VCQUxzRXZPVm0KV21OcFhhT3czc0dVWnp6TlQvYWlJYTY2U2toc0JKSm16TmYxdU9mN0l6WVhCYXRiYUZiU2dXNHh0cFFuWjNETQovWE1vSllBQXk5UlFSMnJWckl2S3ZaQ2ZkWFh1eElPbDhXbFJvVHhONWQxMXBXQVBHeGRnTitXOWhNMEdER25VCmpLbDVUYjU0Z0Q1NzN1OENWWVNwSlNSWWpqd2pib3RIN0Nsamo1UVpMbCsvVFVmYkFjcEovNGl4OW5FcUJMbVcKWllvVHdhb01INFo3amJ0ZlFVa3cxWGkwNXJnNS9qdGYxclVpOHhrNmY3eDlvU2JpTWIvTmxLdU1LeXNJb3RkQwo1RkdyZUloOFNrcktEQTdXd29OdnY3WjBvR3hwMzBSRmJwcEdoYzlhNlloTU1uWHVrMllwRjlPTzViSDFrQmVYCkdzazI1U0JpMUpLWDBYRUNBd0VBQWFOL01IMHdEZ1lEVlIwUEFRSC9CQVFEQWdXZ01CMEdBMVVkSlFRV01CUUcKQ0NzR0FRVUZCd01CQmdnckJnRUZCUWNEQWpBTUJnTlZIUk1CQWY4RUFqQUFNQjBHQTFVZERnUVdCQlI0eTBNdApJRW5NbFFFT3hkUWlJemFISlBZYWhqQWZCZ05WSFNNRUdEQVdnQlRjVHlpaHV0YXNxcndVT0tVNHpiVHhzVWhsCkJEQU5CZ2txaGtpRzl3MEJBUXNGQUFPQ0FRRUFIS3ZoQnJlTnhGRzZFMGhIWk81NUQxZnMvbzllMWdKcGtCWnEKckl3NXlqUjRqUTdVdFhDVTRndXh1Vzl3MXNOcE1qc0hvRWx5Z3dnQ0N4Z0RGT2pUUzM2aU0wMTFrdE11dzRLNQpQdE1BdFlINThKWXA4NVFlazI1ZXRheEVvbENxcWZPWGxBRmJDZ2lOZ2ttdmJUbXU3dXJLLzFCNGExMkRsd1pPCm1reHhBb29aOWZuREZsK2oyckFCc1o2VS9peDlpVWVYejZTeTRtSHY2THB5OUpSbXYyTGpIRy9yc2xRUHp6YzcKelhoQm1CZEJRNk9TTmZJKzFXMmZCY0VMS3oyeXhycGVxS0dybnJuVVhUSkxhLzF0NjZTYTN6VEljdXg2WG11RQpmckIyOHBnMXBxV1BhVk5YQVpxdk1MZzVCY1NPcGJJaWpmZ1JLcXM2RVVNUXMvTEcvQT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS4K
    client-key-data: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcFFJQkFBS0NBUUVBdXdTODVXWmFZMmxkbzdEZXdaUm5QTTFQOXFJaHJycEtTR3dFa21iTTEvVzQ1L3NqCk5oY0ZxMXRvVnRLQmJqRzJsQ2RuY016OWN5Z2xnQURMMUZCSGF0V3NpOHE5a0o5MWRlN0VnNlh4YVZHaFBFM2wKM1hXbFlBOGJGMkEzNWIyRXpRWU1hZFNNcVhsTnZuaUFQbnZlN3dKVmhLa2xKRmlPUENOdWkwZnNLV09QbEJrdQpYNzlOUjlzQnlrbi9pTEgyY1NvRXVaWmxpaFBCcWd3ZmhudU51MTlCU1REVmVMVG11RG4rTzEvV3RTTHpHVHAvCnZIMmhKdUl4djgyVXE0d3JLd2lpMTBMa1VhdDRpSHhLU3NvTUR0YkNnMisvdG5TZ2JHbmZSRVZ1bWthRnoxcnAKaUV3eWRlNlRaaWtYMDQ3bHNmV1FGNWNheVRibElHTFVrcGZSY1FJREFRQUJBb0lCQUhYTGl5czJwOUliNkxZVQp6b25CYnJFMlpKcGxEckFlZUhGYlVCbmlsRDJtY1J2MDYvM0N6SGhkTDhBWUFSd21SZWpWVk9zUXdzY0l6MjNyCmtuY2RSWTUrSFp0RFRObE9CczhNWUV6SGRlSXZYMDQ3aG9CUi9LTWZnS0hkb2ZlYndvemN0VzduU04zcUlOVEsKMDRRSHc2aHBvUEhaRkNMcmdGTlN3ZXNLbHk2TmlSVzdJN0p3UThCczVNME9iMXpUTjNVL2NkYkNmeHJFUTYwVwpkY2dKUmhjMFJyaFZrY3dnREd0T0hFVGt4NHk1UlFhWFhEZEd4a1hRRGwrczdoblM1azVnSGtIcXlLM3VKdnczCkdFejJ6MXMxNmt0U2kwaUpWajJ0MmZoU2N5UTFQdzg2OURxbWZOOFFKNW44dEp5YmJkNnYrSWwxVWsrNlFocG0KbXJyVFVBRUNnWUVBOHdsaFJXVk1DN3pyWm5IWFB3S2Y1ekF3S1k1c2ZBaVh6U05zalluRTZHN1BpaUZ3cllVVwpWUzUvNjNXZlRsalJZdlBrb2FrUzJSbHl5aDVQV1hmNmgxRDhTZmFCTlRRTFgvQk5WcG1ueFUvNTNFenVpcWxXCkNKWVI5Q2t2Y1NEQkJxSlRDK2l3WFRGVUwxdUhRRzNBRU53SWdFVGxhcjhaK0lIL1BraTdMUUVDZ1lFQXhQNXcKMUdqdkNuRkp0YjJBNVVFNHpNTXpMWXRtVGgxR1czcG1IKzI1UGxFZUM0SE1LV2VjVzNtL2U5M1FldzRLdDJXegpVbldQSUUrNnRHZEIybGF1MUtVZ1lZWGRQQWhVYkdibmVXQWJwUTRBU0hWeVlBMm51NUpGNHRHNnhjeEdTWkpYCmRKczRQSnAvR1dRS1N2MC9PL2dkOHN2aEdlQ2V2VVZWbTQwVTlIRUNnWUVBNmp5VUg0b3Q3UEk3L3hTaFcvMXYKbUNaOWhNL2NCdjlSTDBtQkNqbEtLcXNDSkNOdXNnNmZJNklaY0JxQlc4V0dxVlJmZXQrMVpzQjhQZ2xRZU82Rgo2MzFHYXhMR0hUejM2Wk4xTm80SmdNWkFEdStteU1YRVFhcEJ5NDBXU0haRkU5dkhKcWN4cytBalB6RjcvY0RKCmFIWnBTeGNiOWZJUldjNFE3enF5REFFQ2dZRUFoM01FRmFrSkk3NzlsYTcxWDZ6VzUwUVlmbXBwTDdERlhjVHQKVDJyZmdrKzRQdVZDZ2YyeDd0dnBvN3ZDeTdtOStKZy9FcVd1Z2VNUVYxYmdXc1piYys4T01zQWVmRmFsNWR0agpzWHM1eHVXM29CclJSK1pidkljNDhscU85ODRiVGg4SGJ6QURIUGlHQitsWGduUmE5RnNJREpmTzhVSVhJOEQyCnVmdnB1cUVDZ1lFQXo4bjcvbnd1UlJ0V3cwZXNTT2RSTGJYZVRSVEgwZHVNWkJSLzhDcWVTS0srcnJFWXdlU3oKVy9TMllJb3dkR1N0OUt6amROWUc3RHhITjE1aDRSMmh3cDRpN2hrYlN3ZlQvUzJBaVVqNEZxM1pSWlhZd3JMQQo2RVNEeXZxS0lZQjNCcUUrcnV4SHlYT1BLd2VPYi8xL2NWV3FuWFBlSHNUWTRleXM4NSsxOHhvPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQi=
`
)

func TestClusterCRD(t *testing.T) {
	cluster1 := &otev1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "c1",
		},
	}
	cluster2 := &otev1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "c2",
		},
	}
	client := oteclient.NewSimpleClientset(cluster1)

	clusterCRD := NewClusterCRD(client)
	assert.NotNil(t, clusterCRD)
	o := clusterCRD.Get("default", "c1")
	assert.NotNil(t, o)

	o = clusterCRD.Get("default", "c2")
	assert.Nil(t, o)
	clusterCRD.Create(cluster2)
	o = clusterCRD.Get("default", "c2")
	assert.NotNil(t, o)

	clusterCRD.Delete(cluster1)
	o = clusterCRD.Get("default", "c1")
	assert.Nil(t, o)

	set := &otev1.Cluster{
		ObjectMeta: cluster2.ObjectMeta,
		Status: otev1.ClusterStatus{
			Timestamp: 11111111,
			Status:    otev1.ClusterStatusOffline,
		},
	}

	err := clusterCRD.UpdateStatus(set)
	assert.Nil(t, err)
	o = clusterCRD.Get(set.Namespace, set.Name)
	assert.NotNil(t, o)
	assert.Equal(t, set.Status, o.Status)
}

func TestClusterControllerCRD(t *testing.T) {
	clustercontroller1 := &otev1.ClusterController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cc1",
		},
	}
	client := oteclient.NewSimpleClientset(clustercontroller1)

	clustercontrollerCRD := NewClusterControllerCRD(client)
	assert.NotNil(t, clustercontrollerCRD)
	o := clustercontrollerCRD.Get("default", "cc1")
	assert.NotNil(t, o)

	o.Spec.Destination = "isset"
	clustercontrollerCRD.Update(o)
	o = clustercontrollerCRD.Get("default", "cc1")
	assert.Equal(t, "isset", o.Spec.Destination)
}

func TestK8sClient(t *testing.T) {
	// kubeConfig file is not exist
	kubeConfig := "notexist"
	i, err := NewClient(kubeConfig)
	assert.NotNil(t, err)
	assert.Nil(t, i)
	k, err := NewK8sClient(kubeConfig)
	assert.NotNil(t, err)
	assert.Nil(t, k)

	// kubeConfigfile is exist
	kubeConfig = "exist"
	writeKubeConfig(t, kubeConfig)
	i, err = NewClient(kubeConfig)
	assert.Nil(t, err)
	assert.NotNil(t, i)
	k, err = NewK8sClient(kubeConfig)
	assert.Nil(t, err)
	assert.NotNil(t, k)
	// remove exist file
	err = os.Remove(kubeConfig)
	assert.Nil(t, err)
}

func writeKubeConfig(t *testing.T, kubeConfig string) {
	err := ioutil.WriteFile(kubeConfig, []byte(kubeConfigBody), 0666)
	assert.Nil(t, err)
}
