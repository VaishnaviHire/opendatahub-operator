package actions

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func MergeDeployments(source *unstructured.Unstructured, target *unstructured.Unstructured) error {

	containersPath := []string{"spec", "template", "spec", "containers"}
	replicasPath := []string{"spec", "replicas"}

	//
	// Resources
	//

	sc, ok, err := unstructured.NestedFieldNoCopy(source.Object, containersPath...)
	if err != nil && ok {
		return err
	}
	tc, ok, err := unstructured.NestedFieldNoCopy(target.Object, containersPath...)
	if err != nil && ok {
		return err
	}

	resources := make(map[string]interface{})

	var sourceContainers []interface{}
	if sc != nil {
		sourceContainers = sc.([]interface{})
	}

	var targetContainers []interface{}
	if tc != nil {
		targetContainers = tc.([]interface{})
	}

	for i := range sourceContainers {
		m := sourceContainers[i].(map[string]interface{})
		name, ok := m["name"]
		if !ok {
			// can't deal with unnamed containers
			continue
		}

		r, ok := m["resources"]
		if !ok {
			r = make(map[string]interface{})
		}

		resources[name.(string)] = r
	}

	for i := range targetContainers {
		m := targetContainers[i].(map[string]interface{})
		name, ok := m["name"]
		if !ok {
			// can't deal with unnamed containers
			continue
		}

		nr, ok := resources[name.(string)]
		if !ok {
			continue
		}

		if len(nr.(map[string]interface{})) == 0 {
			delete(m, "resources")
		} else {
			m["resources"] = nr
		}
	}

	//
	// Replicas
	//

	sourceReplica, ok, err := unstructured.NestedFieldNoCopy(source.Object, replicasPath...)
	if err != nil {
		return err
	}
	if !ok {
		unstructured.RemoveNestedField(target.Object, replicasPath...)
	} else {
		if err := unstructured.SetNestedField(target.Object, sourceReplica, replicasPath...); err != nil {
			return err
		}
	}

	return nil
}
