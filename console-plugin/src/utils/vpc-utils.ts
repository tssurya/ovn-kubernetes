import { K8sResourceCommon } from '@openshift-console/dynamic-plugin-sdk';

export interface VPCSubnet {
  name: string;
  cidrs: string[];
  type?: 'Public' | 'Private' | 'Isolated' | 'VPNOnly';
  availabilityZone?: {
    clusterSelector: { matchLabels?: Record<string, string> };
    nodeSelector?: Record<string, string>;
  };
}

export interface VPCCondition {
  type: string;
  status: string;
  reason?: string;
  message?: string;
  lastTransitionTime?: string;
}

export interface VPCResource extends K8sResourceCommon {
  spec: {
    subnets: VPCSubnet[];
  };
  status?: {
    subnetCount?: number;
    conditions?: VPCCondition[];
  };
}

export const SUBNET_TYPES = ['Public', 'Private', 'Isolated', 'VPNOnly'] as const;

export function getVPCReadyCondition(vpc: VPCResource): VPCCondition | undefined {
  return vpc.status?.conditions?.find((c) => c.type === 'Ready');
}

export function getVPCStatusLabel(vpc: VPCResource): string {
  const ready = getVPCReadyCondition(vpc);
  if (!ready) return 'Unknown';
  return ready.status === 'True' ? 'Ready' : ready.reason || 'Not Ready';
}

export function isVPCReady(vpc: VPCResource): boolean {
  const ready = getVPCReadyCondition(vpc);
  return ready?.status === 'True';
}

export function timeAgo(dateStr: string | undefined): string {
  if (!dateStr) return '-';
  const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  return `${days}d`;
}
