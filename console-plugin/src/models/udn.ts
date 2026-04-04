import { K8sModel } from '@openshift-console/dynamic-plugin-sdk';

export const UDNModel: K8sModel = {
  apiGroup: 'k8s.ovn.org',
  apiVersion: 'v1',
  kind: 'UserDefinedNetwork',
  plural: 'userdefinednetworks',
  label: 'UserDefinedNetwork',
  labelPlural: 'UserDefinedNetworks',
  abbr: 'UDN',
  namespaced: true,
};

export const udnGroupVersionKind = {
  group: 'k8s.ovn.org',
  version: 'v1',
  kind: 'UserDefinedNetwork',
};
