apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  creationTimestamp: null
  labels:
  name: edgenodes.ote.baidu.com
spec:
  group: ote.baidu.com
  names:
    kind: EdgeNode
    plural: edgenodes
    shortNames:
    - en
    singular: edgenode
  scope: Namespaced
  additionalPrinterColumns:
    - name: Status
      type: string
      JSONPath: .status
    - name: Age
      type: date
      JSONPath: .metadata.creationTimestamp
  validation:
    openAPIV3Schema:
      properties:
        spec:
          properties:
            name:
              type: string
  version: v1
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: ote-crd
rules:
- apiGroups:
  - ote.baidu.com
  resources:
  - customresourcedefinitions
  verbs:
  - create
- apiGroups:
  - ote.baidu.com
  resources:
  - edgenodes
  verbs:
  - list
  - get
  - update
  - watch
  - create
  - delete
