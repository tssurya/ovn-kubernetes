import * as React from 'react';
import {
  useK8sWatchResource,
  K8sResourceCommon,
} from '@openshift-console/dynamic-plugin-sdk';
import {
  Page,
  PageSection,
  Title,
  Spinner,
  EmptyState,
  EmptyStateBody,
  Label,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';

import { raGroupVersionKind } from '../models/ra';
import { timeAgo } from '../utils/vpc-utils';

interface NetworkSelector {
  networkSelectionType?: string;
  primaryUserDefinedNetworkSelector?: {
    namespaceSelector?: { matchLabels?: Record<string, string> };
  };
  clusterUserDefinedNetworkSelector?: {
    networkSelector?: { matchLabels?: Record<string, string> };
  };
}

interface RAResource extends K8sResourceCommon {
  spec?: {
    targetVRF?: string;
    networkSelectors?: NetworkSelector[];
    advertisements?: string[];
  };
  status?: {
    status?: string;
    conditions?: Array<{
      type: string;
      status: string;
      reason?: string;
      message?: string;
    }>;
  };
}

function formatNetworkSelectors(selectors: NetworkSelector[] | undefined): string {
  if (!selectors || selectors.length === 0) return '-';
  return selectors.map((s) => {
    const type = s.networkSelectionType || 'Unknown';
    if (s.primaryUserDefinedNetworkSelector?.namespaceSelector?.matchLabels) {
      const labels = s.primaryUserDefinedNetworkSelector.namespaceSelector.matchLabels;
      const labelStr = Object.entries(labels).map(([k, v]) => `${k}=${v}`).join(', ');
      return `${type} (${labelStr})`;
    }
    if (s.clusterUserDefinedNetworkSelector?.networkSelector?.matchLabels) {
      const labels = s.clusterUserDefinedNetworkSelector.networkSelector.matchLabels;
      const labelStr = Object.entries(labels).map(([k, v]) => `${k}=${v}`).join(', ');
      return `${type} (${labelStr})`;
    }
    return type;
  }).join('; ');
}

function getRAStatus(ra: RAResource): { label: string; color: 'green' | 'orange' | 'grey' } {
  const statusStr = ra.status?.status;
  if (statusStr === 'Success') return { label: 'Success', color: 'green' };
  const accepted = ra.status?.conditions?.find((c) => c.type === 'Accepted');
  if (accepted?.status === 'True') return { label: 'Accepted', color: 'green' };
  if (accepted) return { label: accepted.reason || 'Not Accepted', color: 'orange' };
  return { label: statusStr || 'Pending', color: 'grey' };
}

const RAListPage: React.FC = () => {
  const [ras, loaded, loadError] = useK8sWatchResource<K8sResourceCommon[]>({
    groupVersionKind: raGroupVersionKind,
    isList: true,
  });

  const raList = (ras as RAResource[]) || [];

  if (loadError) {
    return (
      <Page>
        <PageSection>
          <EmptyState>
            <EmptyStateBody>Error loading RouteAdvertisements: {loadError.message || String(loadError)}</EmptyStateBody>
          </EmptyState>
        </PageSection>
      </Page>
    );
  }

  if (!loaded) {
    return (
      <Page>
        <PageSection><Spinner size="xl" /></PageSection>
      </Page>
    );
  }

  return (
    <Page>
      <PageSection variant="default">
        <Title headingLevel="h1">RouteAdvertisements</Title>
      </PageSection>
      <PageSection>
        {raList.length === 0 ? (
          <EmptyState>
            <Title headingLevel="h2" size="lg">No RouteAdvertisements found</Title>
            <EmptyStateBody>
              RouteAdvertisements are created automatically by the VPC controller for Public subnets.
            </EmptyStateBody>
          </EmptyState>
        ) : (
          <Table aria-label="RouteAdvertisements" variant="compact">
            <Thead>
              <Tr>
                <Th>Name</Th>
                <Th>Network Selectors</Th>
                <Th>Advertisements</Th>
                <Th>Target VRF</Th>
                <Th>Status</Th>
                <Th>Age</Th>
              </Tr>
            </Thead>
            <Tbody>
              {raList.map((ra) => {
                const status = getRAStatus(ra);
                return (
                  <Tr key={ra.metadata?.name}>
                    <Td dataLabel="Name">{ra.metadata?.name}</Td>
                    <Td dataLabel="Network Selectors">
                      {formatNetworkSelectors(ra.spec?.networkSelectors)}
                    </Td>
                    <Td dataLabel="Advertisements">
                      {ra.spec?.advertisements?.map((a) => (
                        <Label key={a} color="blue" style={{ marginRight: '0.25rem' }}>{a}</Label>
                      )) || '-'}
                    </Td>
                    <Td dataLabel="Target VRF">{ra.spec?.targetVRF || '-'}</Td>
                    <Td dataLabel="Status">
                      <Label color={status.color}>{status.label}</Label>
                    </Td>
                    <Td dataLabel="Age">{timeAgo(ra.metadata?.creationTimestamp)}</Td>
                  </Tr>
                );
              })}
            </Tbody>
          </Table>
        )}
      </PageSection>
    </Page>
  );
};

export default RAListPage;
