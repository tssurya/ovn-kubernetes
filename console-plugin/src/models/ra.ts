import { K8sModel } from '@openshift-console/dynamic-plugin-sdk';

export const RAModel: K8sModel = {
  apiGroup: 'k8s.ovn.org',
  apiVersion: 'v1',
  kind: 'RouteAdvertisements',
  plural: 'routeadvertisements',
  label: 'RouteAdvertisements',
  labelPlural: 'RouteAdvertisements',
  abbr: 'RA',
  namespaced: false,
};

export const raGroupVersionKind = {
  group: 'k8s.ovn.org',
  version: 'v1',
  kind: 'RouteAdvertisements',
};
