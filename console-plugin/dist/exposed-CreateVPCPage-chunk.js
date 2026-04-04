"use strict";
(self["webpackChunkovn_vpc_console_plugin"] = self["webpackChunkovn_vpc_console_plugin"] || []).push([["exposed-CreateVPCPage"],{

/***/ "./components/CreateVPCPage.tsx"
/*!**************************************!*\
  !*** ./components/CreateVPCPage.tsx ***!
  \**************************************/
(__unused_webpack_module, __webpack_exports__, __webpack_require__) {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   "default": () => (__WEBPACK_DEFAULT_EXPORT__)
/* harmony export */ });
/* harmony import */ var react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(/*! react/jsx-runtime */ "../node_modules/react/jsx-runtime.js");
/* harmony import */ var react__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__(/*! react */ "webpack/sharing/consume/default/react");
/* harmony import */ var react__WEBPACK_IMPORTED_MODULE_1___default = /*#__PURE__*/__webpack_require__.n(react__WEBPACK_IMPORTED_MODULE_1__);
/* harmony import */ var react_router_dom_v5_compat__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__(/*! react-router-dom-v5-compat */ "webpack/sharing/consume/default/react-router-dom-v5-compat");
/* harmony import */ var react_router_dom_v5_compat__WEBPACK_IMPORTED_MODULE_2___default = /*#__PURE__*/__webpack_require__.n(react_router_dom_v5_compat__WEBPACK_IMPORTED_MODULE_2__);
/* harmony import */ var _openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_3__ = __webpack_require__(/*! @openshift-console/dynamic-plugin-sdk */ "webpack/sharing/consume/default/@openshift-console/dynamic-plugin-sdk");
/* harmony import */ var _openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_3___default = /*#__PURE__*/__webpack_require__.n(_openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_3__);
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_4__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Page */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Page/@patternfly/react-core/dist/dynamic/components/Page");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Title__WEBPACK_IMPORTED_MODULE_5__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Title */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Title/@patternfly/react-core/dist/dynamic/components/Title");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Breadcrumb__WEBPACK_IMPORTED_MODULE_6__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Breadcrumb */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Breadcrumb/@patternfly/react-core/dist/dynamic/components/Breadcrumb");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Wizard__WEBPACK_IMPORTED_MODULE_7__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Wizard */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Wizard/@patternfly/react-core/dist/dynamic/components/Wizard");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_8__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Form */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Form/@patternfly/react-core/dist/dynamic/components/Form");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_TextInput__WEBPACK_IMPORTED_MODULE_9__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/TextInput */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/TextInput/@patternfly/react-core/dist/dynamic/components/TextInput");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_FormSelect__WEBPACK_IMPORTED_MODULE_10__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/FormSelect */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/FormSelect/@patternfly/react-core/dist/dynamic/components/FormSelect");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_11__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Button */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Button/@patternfly/react-core/dist/dynamic/components/Button");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Alert__WEBPACK_IMPORTED_MODULE_12__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Alert */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Alert/@patternfly/react-core/dist/dynamic/components/Alert");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_HelperText__WEBPACK_IMPORTED_MODULE_13__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/HelperText */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/HelperText/@patternfly/react-core/dist/dynamic/components/HelperText");
/* harmony import */ var _models_vpc__WEBPACK_IMPORTED_MODULE_14__ = __webpack_require__(/*! ../models/vpc */ "./models/vpc.ts");
/* harmony import */ var _utils_vpc_utils__WEBPACK_IMPORTED_MODULE_15__ = __webpack_require__(/*! ../utils/vpc-utils */ "./utils/vpc-utils.ts");
























const CreateVPCPage = () => {
    const navigate = (0,react_router_dom_v5_compat__WEBPACK_IMPORTED_MODULE_2__.useNavigate)();
    const [vpcName, setVpcName] = react__WEBPACK_IMPORTED_MODULE_1__.useState('');
    const [subnetName, setSubnetName] = react__WEBPACK_IMPORTED_MODULE_1__.useState('');
    const [cidr, setCidr] = react__WEBPACK_IMPORTED_MODULE_1__.useState('');
    const [subnetType, setSubnetType] = react__WEBPACK_IMPORTED_MODULE_1__.useState('Private');
    const [error, setError] = react__WEBPACK_IMPORTED_MODULE_1__.useState(null);
    const [creating, setCreating] = react__WEBPACK_IMPORTED_MODULE_1__.useState(false);
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
            await (0,_openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_3__.k8sCreate)({ model: _models_vpc__WEBPACK_IMPORTED_MODULE_14__.VPCModel, data: resource });
            navigate(`/k8s/cluster/vpcs/${vpcName}`);
        }
        catch (e) {
            const msg = e instanceof Error ? e.message : String(e);
            setError(msg);
            setCreating(false);
        }
    };
    return ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_4__.Page, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_4__.PageSection, { variant: "default", children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Breadcrumb__WEBPACK_IMPORTED_MODULE_6__.Breadcrumb, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Breadcrumb__WEBPACK_IMPORTED_MODULE_6__.BreadcrumbItem, { to: "/k8s/cluster/vpcs", onClick: (e) => { e.preventDefault(); navigate('/k8s/cluster/vpcs'); }, children: "VPCs" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Breadcrumb__WEBPACK_IMPORTED_MODULE_6__.BreadcrumbItem, { isActive: true, children: "Create VPC" })] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Title__WEBPACK_IMPORTED_MODULE_5__.Title, { headingLevel: "h1", style: { marginTop: '1rem' }, children: "Create VPC" })] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_4__.PageSection, { children: [error && ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Alert__WEBPACK_IMPORTED_MODULE_12__.Alert, { variant: "danger", title: "Creation failed", style: { marginBottom: '1rem' }, children: error })), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Wizard__WEBPACK_IMPORTED_MODULE_7__.Wizard, { height: 400, title: "Create VPC", children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Wizard__WEBPACK_IMPORTED_MODULE_7__.WizardStep, { name: "VPC Details", id: "vpc-details", children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_8__.Form, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_8__.FormGroup, { label: "VPC Name", isRequired: true, fieldId: "vpc-name", children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_TextInput__WEBPACK_IMPORTED_MODULE_9__.TextInput, { id: "vpc-name", value: vpcName, onChange: (_e, val) => setVpcName(val), isRequired: true, validated: vpcName.length === 0 ? 'default' : nameValid ? 'success' : 'error' }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_8__.FormHelperText, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_HelperText__WEBPACK_IMPORTED_MODULE_13__.HelperText, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_HelperText__WEBPACK_IMPORTED_MODULE_13__.HelperTextItem, { children: "Lowercase alphanumeric, may contain hyphens. Max 63 characters." }) }) })] }) }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Wizard__WEBPACK_IMPORTED_MODULE_7__.WizardStep, { name: "First Subnet", id: "first-subnet", footer: {
                                    nextButtonText: 'Create',
                                    onNext: handleCreate,
                                    isNextDisabled: !nameValid || !subnetNameValid || !cidrValid || creating,
                                }, children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_8__.Form, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_8__.FormGroup, { label: "Subnet Name", isRequired: true, fieldId: "subnet-name", children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_TextInput__WEBPACK_IMPORTED_MODULE_9__.TextInput, { id: "subnet-name", value: subnetName, onChange: (_e, val) => setSubnetName(val), isRequired: true, validated: subnetName.length === 0
                                                        ? 'default'
                                                        : subnetNameValid
                                                            ? 'success'
                                                            : 'error' }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_8__.FormHelperText, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_HelperText__WEBPACK_IMPORTED_MODULE_13__.HelperText, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_HelperText__WEBPACK_IMPORTED_MODULE_13__.HelperTextItem, { children: ["Namespace will be created as ", vpcName || '<vpc>', "-", subnetName || '<subnet>', "."] }) }) })] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_8__.FormGroup, { label: "CIDRs", isRequired: true, fieldId: "subnet-cidrs", children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_TextInput__WEBPACK_IMPORTED_MODULE_9__.TextInput, { id: "subnet-cidrs", value: cidr, onChange: (_e, val) => setCidr(val), placeholder: "10.100.0.0/16", isRequired: true }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_8__.FormHelperText, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_HelperText__WEBPACK_IMPORTED_MODULE_13__.HelperText, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_HelperText__WEBPACK_IMPORTED_MODULE_13__.HelperTextItem, { children: "Comma-separated CIDR ranges (e.g. 10.100.0.0/16, fd00:100::/48)." }) }) })] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_8__.FormGroup, { label: "Type", fieldId: "subnet-type", children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_FormSelect__WEBPACK_IMPORTED_MODULE_10__.FormSelect, { id: "subnet-type", value: subnetType, onChange: (_e, val) => setSubnetType(val), children: _utils_vpc_utils__WEBPACK_IMPORTED_MODULE_15__.SUBNET_TYPES.map((t) => ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_FormSelect__WEBPACK_IMPORTED_MODULE_10__.FormSelectOption, { value: t, label: t }, t))) }) })] }) })] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_8__.ActionGroup, { style: { marginTop: '1rem' }, children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_11__.Button, { variant: "link", onClick: () => navigate('/k8s/cluster/vpcs'), children: "Cancel" }) })] })] }));
};
/* harmony default export */ const __WEBPACK_DEFAULT_EXPORT__ = (CreateVPCPage);


/***/ },

/***/ "./models/vpc.ts"
/*!***********************!*\
  !*** ./models/vpc.ts ***!
  \***********************/
(__unused_webpack_module, __webpack_exports__, __webpack_require__) {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   VPCModel: () => (/* binding */ VPCModel),
/* harmony export */   vpcGroupVersionKind: () => (/* binding */ vpcGroupVersionKind)
/* harmony export */ });
const VPCModel = {
    apiGroup: 'k8s.ovn.org',
    apiVersion: 'v1beta1',
    kind: 'VPC',
    plural: 'vpcs',
    label: 'VPC',
    labelPlural: 'VPCs',
    abbr: 'VPC',
    namespaced: false,
};
const vpcGroupVersionKind = {
    group: 'k8s.ovn.org',
    version: 'v1beta1',
    kind: 'VPC',
};


/***/ },

/***/ "./utils/vpc-utils.ts"
/*!****************************!*\
  !*** ./utils/vpc-utils.ts ***!
  \****************************/
(__unused_webpack_module, __webpack_exports__, __webpack_require__) {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   SUBNET_TYPES: () => (/* binding */ SUBNET_TYPES),
/* harmony export */   getVPCReadyCondition: () => (/* binding */ getVPCReadyCondition),
/* harmony export */   getVPCStatusLabel: () => (/* binding */ getVPCStatusLabel),
/* harmony export */   isVPCReady: () => (/* binding */ isVPCReady),
/* harmony export */   timeAgo: () => (/* binding */ timeAgo)
/* harmony export */ });
const SUBNET_TYPES = ['Public', 'Private', 'Isolated', 'VPNOnly'];
function getVPCReadyCondition(vpc) {
    return vpc.status?.conditions?.find((c) => c.type === 'Ready');
}
function getVPCStatusLabel(vpc) {
    const ready = getVPCReadyCondition(vpc);
    if (!ready)
        return 'Unknown';
    return ready.status === 'True' ? 'Ready' : ready.reason || 'Not Ready';
}
function isVPCReady(vpc) {
    const ready = getVPCReadyCondition(vpc);
    return ready?.status === 'True';
}
function timeAgo(dateStr) {
    if (!dateStr)
        return '-';
    const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
    if (seconds < 60)
        return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60)
        return `${minutes}m`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24)
        return `${hours}h`;
    const days = Math.floor(hours / 24);
    return `${days}d`;
}


/***/ }

}]);
//# sourceMappingURL=exposed-CreateVPCPage-chunk.js.map