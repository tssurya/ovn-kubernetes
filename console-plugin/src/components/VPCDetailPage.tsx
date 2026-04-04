import * as React from 'react';
import { useParams, useNavigate } from 'react-router-dom-v5-compat';
import {
  useK8sWatchResource,
  k8sDelete,
  k8sPatch,
  K8sResourceCommon,
} from '@openshift-console/dynamic-plugin-sdk';
import {
  Page,
  PageSection,
  Title,
  Breadcrumb,
  BreadcrumbItem,
  Label,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Tabs,
  Tab,
  TabTitleText,
  Button,
  Spinner,
  EmptyState,
  EmptyStateBody,
  Modal,
  ModalVariant,
  Split,
  SplitItem,
  ActionGroup,
  Form,
  FormGroup,
  TextInput,
  FormSelect,
  FormSelectOption,
  Alert,
  FormHelperText,
  HelperText,
  HelperTextItem,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';

import { VPCModel, vpcGroupVersionKind } from '../models/vpc';
import {
  VPCResource,
  VPCCondition,
  SUBNET_TYPES,
  getVPCStatusLabel,
  isVPCReady,
  timeAgo,
} from '../utils/vpc-utils';

const VPCDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = React.useState<string | number>('subnets');
  const [deleteOpen, setDeleteOpen] = React.useState(false);
  const [deleting, setDeleting] = React.useState(false);
  const [addSubnetOpen, setAddSubnetOpen] = React.useState(false);
  const [subnetName, setSubnetName] = React.useState('');
  const [subnetCidr, setSubnetCidr] = React.useState('');
  const [subnetType, setSubnetType] = React.useState('Private');
  const [addError, setAddError] = React.useState<string | null>(null);
  const [adding, setAdding] = React.useState(false);
  const [removeTarget, setRemoveTarget] = React.useState<string | null>(null);
  const [removing, setRemoving] = React.useState(false);

  const [resource, loaded, loadError] = useK8sWatchResource<K8sResourceCommon>({
    groupVersionKind: vpcGroupVersionKind,
    name,
  });

  const vpc = resource as VPCResource | undefined;

  const handleDelete = async () => {
    if (!vpc) return;
    setDeleting(true);
    try {
      await k8sDelete({ model: VPCModel, resource: vpc });
      navigate('/k8s/cluster/vpcs');
    } catch (e) {
      console.error('Failed to delete VPC:', e);
      setDeleting(false);
    }
  };

  const handleAddSubnet = async () => {
    if (!vpc) return;
    setAddError(null);
    setAdding(true);
    const cidrs = subnetCidr.split(',').map((c) => c.trim()).filter(Boolean);
    const newSubnet = { name: subnetName, cidrs, type: subnetType };
    const existingSubnets = vpc.spec?.subnets || [];

    try {
      await k8sPatch({
        model: VPCModel,
        resource: vpc,
        data: [
          {
            op: 'replace',
            path: '/spec/subnets',
            value: [...existingSubnets, newSubnet],
          },
        ],
      });
      setAddSubnetOpen(false);
      setSubnetName('');
      setSubnetCidr('');
      setSubnetType('Private');
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      setAddError(msg);
    } finally {
      setAdding(false);
    }
  };

  const handleRemoveSubnet = async () => {
    if (!vpc || !removeTarget) return;
    setRemoving(true);
    const remaining = (vpc.spec?.subnets || []).filter((s) => s.name !== removeTarget);
    try {
      await k8sPatch({
        model: VPCModel,
        resource: vpc,
        data: [
          {
            op: 'replace',
            path: '/spec/subnets',
            value: remaining,
          },
        ],
      });
      setRemoveTarget(null);
    } catch (e) {
      console.error('Failed to remove subnet:', e);
    } finally {
      setRemoving(false);
    }
  };

  if (loadError) {
    return (
      <Page>
        <PageSection>
          <EmptyState>
            <EmptyStateBody>
              Error loading VPC "{name}": {loadError.message || String(loadError)}
            </EmptyStateBody>
          </EmptyState>
        </PageSection>
      </Page>
    );
  }

  if (!loaded || !vpc) {
    return (
      <Page>
        <PageSection>
          <Spinner size="xl" />
        </PageSection>
      </Page>
    );
  }

  const subnets = vpc.spec?.subnets || [];
  const conditions: VPCCondition[] = vpc.status?.conditions || [];
  const subnetNameValid = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(subnetName);

  return (
    <Page>
      <PageSection variant="default">
        <Breadcrumb>
          <BreadcrumbItem
            to="/k8s/cluster/vpcs"
            onClick={(e) => { e.preventDefault(); navigate('/k8s/cluster/vpcs'); }}
          >
            VPCs
          </BreadcrumbItem>
          <BreadcrumbItem isActive>{name}</BreadcrumbItem>
        </Breadcrumb>

        <Split hasGutter style={{ marginTop: '1rem' }}>
          <SplitItem isFilled>
            <Title headingLevel="h1">
              {name}{' '}
              <Label color={isVPCReady(vpc) ? 'green' : 'orange'}>
                {getVPCStatusLabel(vpc)}
              </Label>
            </Title>
          </SplitItem>
          <SplitItem>
            <Button variant="primary" onClick={() => setAddSubnetOpen(true)} style={{ marginRight: '0.5rem' }}>
              Add Subnet
            </Button>
            <Button variant="danger" onClick={() => setDeleteOpen(true)}>
              Delete VPC
            </Button>
          </SplitItem>
        </Split>

        <DescriptionList style={{ marginTop: '1rem' }}>
          <DescriptionListGroup>
            <DescriptionListTerm>Created</DescriptionListTerm>
            <DescriptionListDescription>
              {vpc.metadata?.creationTimestamp
                ? new Date(vpc.metadata.creationTimestamp).toLocaleString()
                : '-'}
            </DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Subnets</DescriptionListTerm>
            <DescriptionListDescription>{subnets.length}</DescriptionListDescription>
          </DescriptionListGroup>
        </DescriptionList>
      </PageSection>

      <PageSection>
        <Tabs activeKey={activeTab} onSelect={(_e, k) => setActiveTab(k)}>
          <Tab eventKey="subnets" title={<TabTitleText>Subnets ({subnets.length})</TabTitleText>}>
            <Table aria-label="Subnets" variant="compact" style={{ marginTop: '1rem' }}>
              <Thead>
                <Tr>
                  <Th>Name</Th>
                  <Th>Type</Th>
                  <Th>CIDRs</Th>
                  <Th>Namespace</Th>
                  <Th />
                </Tr>
              </Thead>
              <Tbody>
                {subnets.map((s) => {
                  const nsName = `${name}-${s.name}`;
                  return (
                    <Tr key={s.name}>
                      <Td dataLabel="Name">{s.name}</Td>
                      <Td dataLabel="Type">
                        <Label
                          color={
                            s.type === 'Public'
                              ? 'blue'
                              : s.type === 'Isolated'
                                ? 'grey'
                                : s.type === 'VPNOnly'
                                  ? 'purple'
                                  : 'green'
                          }
                        >
                          {s.type || 'Private'}
                        </Label>
                      </Td>
                      <Td dataLabel="CIDRs">{s.cidrs?.join(', ') || '-'}</Td>
                      <Td dataLabel="Namespace">{nsName}</Td>
                      <Td dataLabel="Actions">
                        <Button
                          variant="link"
                          isDanger
                          isDisabled={subnets.length <= 1}
                          onClick={() => setRemoveTarget(s.name)}
                        >
                          Remove
                        </Button>
                      </Td>
                    </Tr>
                  );
                })}
              </Tbody>
            </Table>
          </Tab>

          <Tab eventKey="conditions" title={<TabTitleText>Conditions</TabTitleText>}>
            {conditions.length === 0 ? (
              <EmptyState style={{ marginTop: '1rem' }}>
                <EmptyStateBody>No conditions reported yet.</EmptyStateBody>
              </EmptyState>
            ) : (
              <Table aria-label="Conditions" variant="compact" style={{ marginTop: '1rem' }}>
                <Thead>
                  <Tr>
                    <Th>Type</Th>
                    <Th>Status</Th>
                    <Th>Reason</Th>
                    <Th>Message</Th>
                    <Th>Last Transition</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {conditions.map((c) => (
                    <Tr key={c.type}>
                      <Td dataLabel="Type">{c.type}</Td>
                      <Td dataLabel="Status">
                        <Label color={c.status === 'True' ? 'green' : 'orange'}>
                          {c.status}
                        </Label>
                      </Td>
                      <Td dataLabel="Reason">{c.reason || '-'}</Td>
                      <Td dataLabel="Message">{c.message || '-'}</Td>
                      <Td dataLabel="Last Transition">{timeAgo(c.lastTransitionTime)}</Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            )}
          </Tab>

          <Tab eventKey="yaml" title={<TabTitleText>YAML</TabTitleText>}>
            <pre style={{ marginTop: '1rem', padding: '1rem', background: 'var(--pf-t--global--background--color--secondary--default, #f0f0f0)' }}>
              {JSON.stringify(vpc, null, 2)}
            </pre>
          </Tab>
        </Tabs>
      </PageSection>

      {/* Add Subnet Modal */}
      <Modal
        variant={ModalVariant.medium}
        title="Add Subnet"
        isOpen={addSubnetOpen}
        onClose={() => { setAddSubnetOpen(false); setAddError(null); }}
      >
        {addError && (
          <Alert variant="danger" title="Failed to add subnet" style={{ marginBottom: '1rem' }}>
            {addError}
          </Alert>
        )}
        <Form>
          <FormGroup label="Subnet Name" isRequired fieldId="add-subnet-name">
            <TextInput
              id="add-subnet-name"
              value={subnetName}
              onChange={(_e, val) => setSubnetName(val)}
              isRequired
              validated={subnetName.length === 0 ? 'default' : subnetNameValid ? 'success' : 'error'}
            />
            <FormHelperText>
              <HelperText>
                <HelperTextItem>
                  Namespace: {name}-{subnetName || '<name>'}
                </HelperTextItem>
              </HelperText>
            </FormHelperText>
          </FormGroup>
          <FormGroup label="CIDRs" isRequired fieldId="add-subnet-cidrs">
            <TextInput
              id="add-subnet-cidrs"
              value={subnetCidr}
              onChange={(_e, val) => setSubnetCidr(val)}
              placeholder="10.200.0.0/16"
              isRequired
            />
            <FormHelperText>
              <HelperText>
                <HelperTextItem>Comma-separated (e.g. 10.200.0.0/16, fd00:200::/48)</HelperTextItem>
              </HelperText>
            </FormHelperText>
          </FormGroup>
          <FormGroup label="Type" fieldId="add-subnet-type">
            <FormSelect
              id="add-subnet-type"
              value={subnetType}
              onChange={(_e, val) => setSubnetType(val)}
            >
              {SUBNET_TYPES.map((t) => (
                <FormSelectOption key={t} value={t} label={t} />
              ))}
            </FormSelect>
          </FormGroup>
        </Form>
        <ActionGroup style={{ marginTop: '1rem' }}>
          <Button
            variant="primary"
            onClick={handleAddSubnet}
            isLoading={adding}
            isDisabled={!subnetNameValid || subnetCidr.length === 0 || adding}
          >
            Add Subnet
          </Button>
          <Button variant="link" onClick={() => { setAddSubnetOpen(false); setAddError(null); }}>
            Cancel
          </Button>
        </ActionGroup>
      </Modal>

      {/* Remove Subnet Modal */}
      <Modal
        variant={ModalVariant.small}
        title={`Remove subnet "${removeTarget}"?`}
        isOpen={removeTarget !== null}
        onClose={() => setRemoveTarget(null)}
      >
        <p>
          This will remove the subnet and the controller will delete its namespace ({name}-{removeTarget}),
          UDN, and any RouteAdvertisements.
        </p>
        <ActionGroup style={{ marginTop: '1rem' }}>
          <Button variant="danger" onClick={handleRemoveSubnet} isLoading={removing}>
            Remove
          </Button>
          <Button variant="link" onClick={() => setRemoveTarget(null)}>
            Cancel
          </Button>
        </ActionGroup>
      </Modal>

      {/* Delete VPC Modal */}
      <Modal
        variant={ModalVariant.small}
        title={`Delete VPC ${name}?`}
        isOpen={deleteOpen}
        onClose={() => setDeleteOpen(false)}
      >
        <p>This will delete the VPC and all its associated namespaces, UDNs, and RouteAdvertisements.</p>
        <ActionGroup style={{ marginTop: '1rem' }}>
          <Button variant="danger" onClick={handleDelete} isLoading={deleting}>
            Delete
          </Button>
          <Button variant="link" onClick={() => setDeleteOpen(false)}>
            Cancel
          </Button>
        </ActionGroup>
      </Modal>
    </Page>
  );
};

export default VPCDetailPage;
