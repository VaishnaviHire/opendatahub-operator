# DataScienceCluster CRD

This document explains all the fields defined by the DataScienceCluster CRD. This crd is used for deployment of
ODH components like notebooks, serving and training.

## Goals

Following is the list of goals for this crd

- Enable / Disable individual components provided by ODH 
- Allow cluster admins to set controller resources for individual components
- Allow cluster admins to set controller replicas for individual components
- Allow cluster admins to set component specific configurations for individual components
- Allow cluster admins to set Oauth options for all components


## spec.profile

A profile sets the default components and configuration to install for a given
use case. The profile configuration can still be **overriden** by the user on a per
component basis. If not defined, the 'full' profile is used. Valid values are:
- `full` : all components are installed
- `serving` : Components required and not limited to serving are installed
- `training` : Components required and not limited to training are installed
- `workbench` : Components required and not limited to jupyter notebooks are installed

## spec.components

This is a list of different components provided by ODH. Note that omponents is a list of multiple
controllers and custom resources. Every component has the following common fields

- `enabled` : When set to true, all the component resources are deployed.
- `replicas` : When set to an int value, component controllers will scale to the given value
- `resources`: Cluster admin can set this value as defined [here](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#resources).

### spec.components.odhNotebookController

In addition to above fields, `notebooks` has the following component specific fields -
- `notebookImages.managed` : When set to true, will allow users to use notebooks provided by ODH

### spec.components.modelmeshController

In addition to above fields, `serving` has the following component specific fields -
- TBD

### spec.components.dataSciencePipelinesController

In addition to above fields, `training` has the following component specific fields -
- TBD