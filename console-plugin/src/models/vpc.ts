import { K8sModel } from '@openshift-console/dynamic-plugin-sdk';

export const VPCModel: K8sModel = {
  apiGroup: 'k8s.ovn.org',
  apiVersion: 'v1beta1',
  kind: 'VPC',
  plural: 'vpcs',
  label: 'VPC',
  labelPlural: 'VPCs',
  abbr: 'VPC',
  namespaced: false,
};

export const vpcGroupVersionKind = {
  group: 'k8s.ovn.org',
  version: 'v1beta1',
  kind: 'VPC',
};
