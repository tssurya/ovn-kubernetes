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

import { udnGroupVersionKind } from '../models/udn';
import { timeAgo } from '../utils/vpc-utils';

interface UDNResource extends K8sResourceCommon {
  spec?: {
    topology?: string;
    layer2?: { role?: string; subnets?: string[] };
    layer3?: { role?: string; subnets?: Array<{ cidr: string }> };
    transport?: string;
    evpn?: { vtep?: string; macVRF?: { vni?: number } };
  };
  status?: {
    conditions?: Array<{
      type: string;
      status: string;
      reason?: string;
    }>;
  };
}

function getUDNStatus(udn: UDNResource): { label: string; color: 'green' | 'orange' | 'grey' } {
  const ready = udn.status?.conditions?.find((c) => c.type === 'NetworkReady');
  if (!ready) return { label: 'Pending', color: 'grey' };
  return ready.status === 'True'
    ? { label: 'Ready', color: 'green' }
    : { label: ready.reason || 'Not Ready', color: 'orange' };
}

function getSubnets(udn: UDNResource): string {
  if (udn.spec?.layer2?.subnets) return udn.spec.layer2.subnets.join(', ');
  if (udn.spec?.layer3?.subnets) return udn.spec.layer3.subnets.map((s) => s.cidr).join(', ');
  return '-';
}

const UDNListPage: React.FC = () => {
  const [udns, loaded, loadError] = useK8sWatchResource<K8sResourceCommon[]>({
    groupVersionKind: udnGroupVersionKind,
    isList: true,
    namespaced: true,
  });

  const udnList = (udns as UDNResource[]) || [];

  if (loadError) {
    return (
      <Page>
        <PageSection>
          <EmptyState>
            <EmptyStateBody>Error loading UDNs: {loadError.message || String(loadError)}</EmptyStateBody>
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
        <Title headingLevel="h1">UserDefinedNetworks</Title>
      </PageSection>
      <PageSection>
        {udnList.length === 0 ? (
          <EmptyState>
            <Title headingLevel="h2" size="lg">No UserDefinedNetworks found</Title>
            <EmptyStateBody>
              UDNs are created automatically by the VPC controller for each VPC subnet.
            </EmptyStateBody>
          </EmptyState>
        ) : (
          <Table aria-label="UserDefinedNetworks" variant="compact">
            <Thead>
              <Tr>
                <Th>Name</Th>
                <Th>Namespace</Th>
                <Th>Topology</Th>
                <Th>Role</Th>
                <Th>Subnets</Th>
                <Th>Transport</Th>
                <Th>Status</Th>
                <Th>Age</Th>
              </Tr>
            </Thead>
            <Tbody>
              {udnList.map((udn) => {
                const status = getUDNStatus(udn);
                const role = udn.spec?.layer2?.role || udn.spec?.layer3?.role || '-';
                return (
                  <Tr key={`${udn.metadata?.namespace}/${udn.metadata?.name}`}>
                    <Td dataLabel="Name">{udn.metadata?.name}</Td>
                    <Td dataLabel="Namespace">{udn.metadata?.namespace}</Td>
                    <Td dataLabel="Topology">
                      <Label color="blue">{udn.spec?.topology || '-'}</Label>
                    </Td>
                    <Td dataLabel="Role">{role}</Td>
                    <Td dataLabel="Subnets">{getSubnets(udn)}</Td>
                    <Td dataLabel="Transport">
                      {udn.spec?.transport ? (
                        <Label color="purple">{udn.spec.transport}</Label>
                      ) : '-'}
                    </Td>
                    <Td dataLabel="Status">
                      <Label color={status.color}>{status.label}</Label>
                    </Td>
                    <Td dataLabel="Age">{timeAgo(udn.metadata?.creationTimestamp)}</Td>
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

export default UDNListPage;
