import * as React from 'react';
import { useNavigate } from 'react-router-dom-v5-compat';
import {
  useK8sWatchResource,
  K8sResourceCommon,
} from '@openshift-console/dynamic-plugin-sdk';
import {
  Page,
  PageSection,
  Title,
  Button,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  EmptyState,
  EmptyStateBody,
  Spinner,
  Label,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';

import { vpcGroupVersionKind } from '../models/vpc';
import { VPCResource, getVPCStatusLabel, isVPCReady, timeAgo } from '../utils/vpc-utils';

const VPCListPage: React.FC = () => {
  const navigate = useNavigate();
  const [vpcs, loaded, loadError] = useK8sWatchResource<K8sResourceCommon[]>({
    groupVersionKind: vpcGroupVersionKind,
    isList: true,
  });

  const vpcList = (vpcs as VPCResource[]) || [];

  if (loadError) {
    return (
      <Page>
        <PageSection>
          <EmptyState>
            <EmptyStateBody>Error loading VPCs: {loadError.message || String(loadError)}</EmptyStateBody>
          </EmptyState>
        </PageSection>
      </Page>
    );
  }

  if (!loaded) {
    return (
      <Page>
        <PageSection>
          <Spinner size="xl" />
        </PageSection>
      </Page>
    );
  }

  return (
    <Page>
      <PageSection variant="default">
        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <Title headingLevel="h1">Virtual Private Clouds</Title>
            </ToolbarItem>
            <ToolbarItem align={{ default: 'alignEnd' }}>
              <Button variant="primary" onClick={() => navigate('/k8s/cluster/vpcs/~new')}>
                Create VPC
              </Button>
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>
      </PageSection>
      <PageSection>
        {vpcList.length === 0 ? (
          <EmptyState>
            <Title headingLevel="h2" size="lg">No VPCs found</Title>
            <EmptyStateBody>
              Create a VPC to define isolated virtual networks with subnets.
            </EmptyStateBody>
            <Button variant="primary" onClick={() => navigate('/k8s/cluster/vpcs/~new')}>
              Create VPC
            </Button>
          </EmptyState>
        ) : (
          <Table aria-label="VPCs" variant="compact">
            <Thead>
              <Tr>
                <Th>Name</Th>
                <Th>Subnets</Th>
                <Th>Status</Th>
                <Th>Age</Th>
              </Tr>
            </Thead>
            <Tbody>
              {vpcList.map((vpc) => (
                <Tr
                  key={vpc.metadata?.name}
                  isClickable
                  onRowClick={() => navigate(`/k8s/cluster/vpcs/${vpc.metadata?.name}`)}
                >
                  <Td dataLabel="Name">{vpc.metadata?.name}</Td>
                  <Td dataLabel="Subnets">{vpc.spec?.subnets?.length ?? 0}</Td>
                  <Td dataLabel="Status">
                    <Label color={isVPCReady(vpc) ? 'green' : 'orange'}>
                      {getVPCStatusLabel(vpc)}
                    </Label>
                  </Td>
                  <Td dataLabel="Age">{timeAgo(vpc.metadata?.creationTimestamp)}</Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
        )}
      </PageSection>
    </Page>
  );
};

export default VPCListPage;
