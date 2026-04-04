import * as React from 'react';
import { useNavigate } from 'react-router-dom-v5-compat';
import { k8sCreate } from '@openshift-console/dynamic-plugin-sdk';
import {
  Page,
  PageSection,
  Title,
  Breadcrumb,
  BreadcrumbItem,
  Wizard,
  WizardStep,
  Form,
  FormGroup,
  TextInput,
  FormSelect,
  FormSelectOption,
  ActionGroup,
  Button,
  Alert,
  FormHelperText,
  HelperText,
  HelperTextItem,
} from '@patternfly/react-core';

import { VPCModel } from '../models/vpc';
import { SUBNET_TYPES } from '../utils/vpc-utils';

const CreateVPCPage: React.FC = () => {
  const navigate = useNavigate();

  const [vpcName, setVpcName] = React.useState('');
  const [subnetName, setSubnetName] = React.useState('');
  const [cidr, setCidr] = React.useState('');
  const [subnetType, setSubnetType] = React.useState('Private');
  const [error, setError] = React.useState<string | null>(null);
  const [creating, setCreating] = React.useState(false);

  const nameValid = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(vpcName);
  const subnetNameValid = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(subnetName);
  const cidrValid = cidr.length > 0;

  const handleCreate = async () => {
    setError(null);
    setCreating(true);

    const cidrs = cidr.split(',').map((c) => c.trim()).filter(Boolean);

    const resource = {
      apiVersion: 'k8s.ovn.org/v1beta1',
      kind: 'VPC',
      metadata: { name: vpcName },
      spec: {
        subnets: [
          {
            name: subnetName,
            cidrs,
            type: subnetType,
          },
        ],
      },
    };

    try {
      await k8sCreate({ model: VPCModel, data: resource });
      navigate(`/k8s/cluster/vpcs/${vpcName}`);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
      setCreating(false);
    }
  };

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
          <BreadcrumbItem isActive>Create VPC</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1" style={{ marginTop: '1rem' }}>
          Create VPC
        </Title>
      </PageSection>

      <PageSection>
        {error && (
          <Alert variant="danger" title="Creation failed" style={{ marginBottom: '1rem' }}>
            {error}
          </Alert>
        )}

        <Wizard height={400} title="Create VPC">
          <WizardStep name="VPC Details" id="vpc-details">
            <Form>
              <FormGroup label="VPC Name" isRequired fieldId="vpc-name">
                <TextInput
                  id="vpc-name"
                  value={vpcName}
                  onChange={(_e, val) => setVpcName(val)}
                  isRequired
                  validated={vpcName.length === 0 ? 'default' : nameValid ? 'success' : 'error'}
                />
                <FormHelperText>
                  <HelperText>
                    <HelperTextItem>
                      Lowercase alphanumeric, may contain hyphens. Max 63 characters.
                    </HelperTextItem>
                  </HelperText>
                </FormHelperText>
              </FormGroup>
            </Form>
          </WizardStep>

          <WizardStep
            name="First Subnet"
            id="first-subnet"
            footer={{
              nextButtonText: 'Create',
              onNext: handleCreate,
              isNextDisabled: !nameValid || !subnetNameValid || !cidrValid || creating,
            }}
          >
            <Form>
              <FormGroup label="Subnet Name" isRequired fieldId="subnet-name">
                <TextInput
                  id="subnet-name"
                  value={subnetName}
                  onChange={(_e, val) => setSubnetName(val)}
                  isRequired
                  validated={
                    subnetName.length === 0
                      ? 'default'
                      : subnetNameValid
                        ? 'success'
                        : 'error'
                  }
                />
                <FormHelperText>
                  <HelperText>
                    <HelperTextItem>
                      Namespace will be created as {vpcName || '<vpc>'}-{subnetName || '<subnet>'}.
                    </HelperTextItem>
                  </HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="CIDRs" isRequired fieldId="subnet-cidrs">
                <TextInput
                  id="subnet-cidrs"
                  value={cidr}
                  onChange={(_e, val) => setCidr(val)}
                  placeholder="10.100.0.0/16"
                  isRequired
                />
                <FormHelperText>
                  <HelperText>
                    <HelperTextItem>
                      Comma-separated CIDR ranges (e.g. 10.100.0.0/16, fd00:100::/48).
                    </HelperTextItem>
                  </HelperText>
                </FormHelperText>
              </FormGroup>

              <FormGroup label="Type" fieldId="subnet-type">
                <FormSelect
                  id="subnet-type"
                  value={subnetType}
                  onChange={(_e, val) => setSubnetType(val)}
                >
                  {SUBNET_TYPES.map((t) => (
                    <FormSelectOption key={t} value={t} label={t} />
                  ))}
                </FormSelect>
              </FormGroup>
            </Form>
          </WizardStep>
        </Wizard>

        <ActionGroup style={{ marginTop: '1rem' }}>
          <Button variant="link" onClick={() => navigate('/k8s/cluster/vpcs')}>
            Cancel
          </Button>
        </ActionGroup>
      </PageSection>
    </Page>
  );
};

export default CreateVPCPage;
