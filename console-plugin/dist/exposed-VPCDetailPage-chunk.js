"use strict";
(self["webpackChunkovn_vpc_console_plugin"] = self["webpackChunkovn_vpc_console_plugin"] || []).push([["exposed-VPCDetailPage"],{

/***/ "./components/VPCDetailPage.tsx"
/*!**************************************!*\
  !*** ./components/VPCDetailPage.tsx ***!
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
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Label__WEBPACK_IMPORTED_MODULE_7__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Label */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Label/@patternfly/react-core/dist/dynamic/components/Label");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_DescriptionList__WEBPACK_IMPORTED_MODULE_8__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/DescriptionList */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/DescriptionList/@patternfly/react-core/dist/dynamic/components/DescriptionList");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Tabs__WEBPACK_IMPORTED_MODULE_9__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Tabs */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Tabs/@patternfly/react-core/dist/dynamic/components/Tabs");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_10__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Button */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Button/@patternfly/react-core/dist/dynamic/components/Button");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Spinner__WEBPACK_IMPORTED_MODULE_11__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Spinner */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Spinner/@patternfly/react-core/dist/dynamic/components/Spinner");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_12__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/EmptyState */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/EmptyState/@patternfly/react-core/dist/dynamic/components/EmptyState");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Modal__WEBPACK_IMPORTED_MODULE_13__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Modal */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Modal/@patternfly/react-core/dist/dynamic/components/Modal");
/* harmony import */ var _patternfly_react_core_dist_dynamic_layouts_Split__WEBPACK_IMPORTED_MODULE_14__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/layouts/Split */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/layouts/Split/@patternfly/react-core/dist/dynamic/layouts/Split");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_15__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Form */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Form/@patternfly/react-core/dist/dynamic/components/Form");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_TextInput__WEBPACK_IMPORTED_MODULE_16__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/TextInput */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/TextInput/@patternfly/react-core/dist/dynamic/components/TextInput");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_FormSelect__WEBPACK_IMPORTED_MODULE_17__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/FormSelect */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/FormSelect/@patternfly/react-core/dist/dynamic/components/FormSelect");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Alert__WEBPACK_IMPORTED_MODULE_18__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Alert */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Alert/@patternfly/react-core/dist/dynamic/components/Alert");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_HelperText__WEBPACK_IMPORTED_MODULE_19__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/HelperText */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/HelperText/@patternfly/react-core/dist/dynamic/components/HelperText");
/* harmony import */ var _patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__ = __webpack_require__(/*! @patternfly/react-table/dist/dynamic/components/Table */ "webpack/sharing/consume/default/@patternfly/react-table/dist/dynamic/components/Table/@patternfly/react-table/dist/dynamic/components/Table");
/* harmony import */ var _models_vpc__WEBPACK_IMPORTED_MODULE_21__ = __webpack_require__(/*! ../models/vpc */ "./models/vpc.ts");
/* harmony import */ var _utils_vpc_utils__WEBPACK_IMPORTED_MODULE_22__ = __webpack_require__(/*! ../utils/vpc-utils */ "./utils/vpc-utils.ts");











































const VPCDetailPage = () => {
    const { name } = (0,react_router_dom_v5_compat__WEBPACK_IMPORTED_MODULE_2__.useParams)();
    const navigate = (0,react_router_dom_v5_compat__WEBPACK_IMPORTED_MODULE_2__.useNavigate)();
    const [activeTab, setActiveTab] = react__WEBPACK_IMPORTED_MODULE_1__.useState('subnets');
    const [deleteOpen, setDeleteOpen] = react__WEBPACK_IMPORTED_MODULE_1__.useState(false);
    const [deleting, setDeleting] = react__WEBPACK_IMPORTED_MODULE_1__.useState(false);
    const [addSubnetOpen, setAddSubnetOpen] = react__WEBPACK_IMPORTED_MODULE_1__.useState(false);
    const [subnetName, setSubnetName] = react__WEBPACK_IMPORTED_MODULE_1__.useState('');
    const [subnetCidr, setSubnetCidr] = react__WEBPACK_IMPORTED_MODULE_1__.useState('');
    const [subnetType, setSubnetType] = react__WEBPACK_IMPORTED_MODULE_1__.useState('Private');
    const [addError, setAddError] = react__WEBPACK_IMPORTED_MODULE_1__.useState(null);
    const [adding, setAdding] = react__WEBPACK_IMPORTED_MODULE_1__.useState(false);
    const [removeTarget, setRemoveTarget] = react__WEBPACK_IMPORTED_MODULE_1__.useState(null);
    const [removing, setRemoving] = react__WEBPACK_IMPORTED_MODULE_1__.useState(false);
    const [resource, loaded, loadError] = (0,_openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_3__.useK8sWatchResource)({
        groupVersionKind: _models_vpc__WEBPACK_IMPORTED_MODULE_21__.vpcGroupVersionKind,
        name,
    });
    const vpc = resource;
    const handleDelete = async () => {
        if (!vpc)
            return;
        setDeleting(true);
        try {
            await (0,_openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_3__.k8sDelete)({ model: _models_vpc__WEBPACK_IMPORTED_MODULE_21__.VPCModel, resource: vpc });
            navigate('/k8s/cluster/vpcs');
        }
        catch (e) {
            console.error('Failed to delete VPC:', e);
            setDeleting(false);
        }
    };
    const handleAddSubnet = async () => {
        if (!vpc)
            return;
        setAddError(null);
        setAdding(true);
        const cidrs = subnetCidr.split(',').map((c) => c.trim()).filter(Boolean);
        const newSubnet = { name: subnetName, cidrs, type: subnetType };
        const existingSubnets = vpc.spec?.subnets || [];
        try {
            await (0,_openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_3__.k8sPatch)({
                model: _models_vpc__WEBPACK_IMPORTED_MODULE_21__.VPCModel,
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
        }
        catch (e) {
            const msg = e instanceof Error ? e.message : String(e);
            setAddError(msg);
        }
        finally {
            setAdding(false);
        }
    };
    const handleRemoveSubnet = async () => {
        if (!vpc || !removeTarget)
            return;
        setRemoving(true);
        const remaining = (vpc.spec?.subnets || []).filter((s) => s.name !== removeTarget);
        try {
            await (0,_openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_3__.k8sPatch)({
                model: _models_vpc__WEBPACK_IMPORTED_MODULE_21__.VPCModel,
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
        }
        catch (e) {
            console.error('Failed to remove subnet:', e);
        }
        finally {
            setRemoving(false);
        }
    };
    if (loadError) {
        return ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_4__.Page, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_4__.PageSection, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_12__.EmptyState, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_12__.EmptyStateBody, { children: ["Error loading VPC \"", name, "\": ", loadError.message || String(loadError)] }) }) }) }));
    }
    if (!loaded || !vpc) {
        return ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_4__.Page, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_4__.PageSection, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Spinner__WEBPACK_IMPORTED_MODULE_11__.Spinner, { size: "xl" }) }) }));
    }
    const subnets = vpc.spec?.subnets || [];
    const conditions = vpc.status?.conditions || [];
    const subnetNameValid = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(subnetName);
    return ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_4__.Page, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_4__.PageSection, { variant: "default", children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Breadcrumb__WEBPACK_IMPORTED_MODULE_6__.Breadcrumb, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Breadcrumb__WEBPACK_IMPORTED_MODULE_6__.BreadcrumbItem, { to: "/k8s/cluster/vpcs", onClick: (e) => { e.preventDefault(); navigate('/k8s/cluster/vpcs'); }, children: "VPCs" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Breadcrumb__WEBPACK_IMPORTED_MODULE_6__.BreadcrumbItem, { isActive: true, children: name })] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_layouts_Split__WEBPACK_IMPORTED_MODULE_14__.Split, { hasGutter: true, style: { marginTop: '1rem' }, children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_layouts_Split__WEBPACK_IMPORTED_MODULE_14__.SplitItem, { isFilled: true, children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Title__WEBPACK_IMPORTED_MODULE_5__.Title, { headingLevel: "h1", children: [name, ' ', (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Label__WEBPACK_IMPORTED_MODULE_7__.Label, { color: (0,_utils_vpc_utils__WEBPACK_IMPORTED_MODULE_22__.isVPCReady)(vpc) ? 'green' : 'orange', children: (0,_utils_vpc_utils__WEBPACK_IMPORTED_MODULE_22__.getVPCStatusLabel)(vpc) })] }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_layouts_Split__WEBPACK_IMPORTED_MODULE_14__.SplitItem, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_10__.Button, { variant: "primary", onClick: () => setAddSubnetOpen(true), style: { marginRight: '0.5rem' }, children: "Add Subnet" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_10__.Button, { variant: "danger", onClick: () => setDeleteOpen(true), children: "Delete VPC" })] })] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_DescriptionList__WEBPACK_IMPORTED_MODULE_8__.DescriptionList, { style: { marginTop: '1rem' }, children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_DescriptionList__WEBPACK_IMPORTED_MODULE_8__.DescriptionListGroup, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_DescriptionList__WEBPACK_IMPORTED_MODULE_8__.DescriptionListTerm, { children: "Created" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_DescriptionList__WEBPACK_IMPORTED_MODULE_8__.DescriptionListDescription, { children: vpc.metadata?.creationTimestamp
                                            ? new Date(vpc.metadata.creationTimestamp).toLocaleString()
                                            : '-' })] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_DescriptionList__WEBPACK_IMPORTED_MODULE_8__.DescriptionListGroup, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_DescriptionList__WEBPACK_IMPORTED_MODULE_8__.DescriptionListTerm, { children: "Subnets" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_DescriptionList__WEBPACK_IMPORTED_MODULE_8__.DescriptionListDescription, { children: subnets.length })] })] })] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_4__.PageSection, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Tabs__WEBPACK_IMPORTED_MODULE_9__.Tabs, { activeKey: activeTab, onSelect: (_e, k) => setActiveTab(k), children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Tabs__WEBPACK_IMPORTED_MODULE_9__.Tab, { eventKey: "subnets", title: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Tabs__WEBPACK_IMPORTED_MODULE_9__.TabTitleText, { children: ["Subnets (", subnets.length, ")"] }), children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Table, { "aria-label": "Subnets", variant: "compact", style: { marginTop: '1rem' }, children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Thead, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Tr, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Th, { children: "Name" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Th, { children: "Type" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Th, { children: "CIDRs" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Th, { children: "Namespace" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Th, {})] }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Tbody, { children: subnets.map((s) => {
                                            const nsName = `${name}-${s.name}`;
                                            return ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Tr, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Td, { dataLabel: "Name", children: s.name }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Td, { dataLabel: "Type", children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Label__WEBPACK_IMPORTED_MODULE_7__.Label, { color: s.type === 'Public'
                                                                ? 'blue'
                                                                : s.type === 'Isolated'
                                                                    ? 'grey'
                                                                    : s.type === 'VPNOnly'
                                                                        ? 'purple'
                                                                        : 'green', children: s.type || 'Private' }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Td, { dataLabel: "CIDRs", children: s.cidrs?.join(', ') || '-' }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Td, { dataLabel: "Namespace", children: nsName }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Td, { dataLabel: "Actions", children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_10__.Button, { variant: "link", isDanger: true, isDisabled: subnets.length <= 1, onClick: () => setRemoveTarget(s.name), children: "Remove" }) })] }, s.name));
                                        }) })] }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Tabs__WEBPACK_IMPORTED_MODULE_9__.Tab, { eventKey: "conditions", title: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Tabs__WEBPACK_IMPORTED_MODULE_9__.TabTitleText, { children: "Conditions" }), children: conditions.length === 0 ? ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_12__.EmptyState, { style: { marginTop: '1rem' }, children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_12__.EmptyStateBody, { children: "No conditions reported yet." }) })) : ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Table, { "aria-label": "Conditions", variant: "compact", style: { marginTop: '1rem' }, children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Thead, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Tr, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Th, { children: "Type" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Th, { children: "Status" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Th, { children: "Reason" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Th, { children: "Message" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Th, { children: "Last Transition" })] }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Tbody, { children: conditions.map((c) => ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Tr, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Td, { dataLabel: "Type", children: c.type }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Td, { dataLabel: "Status", children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Label__WEBPACK_IMPORTED_MODULE_7__.Label, { color: c.status === 'True' ? 'green' : 'orange', children: c.status }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Td, { dataLabel: "Reason", children: c.reason || '-' }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Td, { dataLabel: "Message", children: c.message || '-' }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_20__.Td, { dataLabel: "Last Transition", children: (0,_utils_vpc_utils__WEBPACK_IMPORTED_MODULE_22__.timeAgo)(c.lastTransitionTime) })] }, c.type))) })] })) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Tabs__WEBPACK_IMPORTED_MODULE_9__.Tab, { eventKey: "yaml", title: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Tabs__WEBPACK_IMPORTED_MODULE_9__.TabTitleText, { children: "YAML" }), children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)("pre", { style: { marginTop: '1rem', padding: '1rem', background: 'var(--pf-t--global--background--color--secondary--default, #f0f0f0)' }, children: JSON.stringify(vpc, null, 2) }) })] }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Modal__WEBPACK_IMPORTED_MODULE_13__.Modal, { variant: _patternfly_react_core_dist_dynamic_components_Modal__WEBPACK_IMPORTED_MODULE_13__.ModalVariant.medium, title: "Add Subnet", isOpen: addSubnetOpen, onClose: () => { setAddSubnetOpen(false); setAddError(null); }, children: [addError && ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Alert__WEBPACK_IMPORTED_MODULE_18__.Alert, { variant: "danger", title: "Failed to add subnet", style: { marginBottom: '1rem' }, children: addError })), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_15__.Form, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_15__.FormGroup, { label: "Subnet Name", isRequired: true, fieldId: "add-subnet-name", children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_TextInput__WEBPACK_IMPORTED_MODULE_16__.TextInput, { id: "add-subnet-name", value: subnetName, onChange: (_e, val) => setSubnetName(val), isRequired: true, validated: subnetName.length === 0 ? 'default' : subnetNameValid ? 'success' : 'error' }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_15__.FormHelperText, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_HelperText__WEBPACK_IMPORTED_MODULE_19__.HelperText, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_HelperText__WEBPACK_IMPORTED_MODULE_19__.HelperTextItem, { children: ["Namespace: ", name, "-", subnetName || '<name>'] }) }) })] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_15__.FormGroup, { label: "CIDRs", isRequired: true, fieldId: "add-subnet-cidrs", children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_TextInput__WEBPACK_IMPORTED_MODULE_16__.TextInput, { id: "add-subnet-cidrs", value: subnetCidr, onChange: (_e, val) => setSubnetCidr(val), placeholder: "10.200.0.0/16", isRequired: true }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_15__.FormHelperText, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_HelperText__WEBPACK_IMPORTED_MODULE_19__.HelperText, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_HelperText__WEBPACK_IMPORTED_MODULE_19__.HelperTextItem, { children: "Comma-separated (e.g. 10.200.0.0/16, fd00:200::/48)" }) }) })] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_15__.FormGroup, { label: "Type", fieldId: "add-subnet-type", children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_FormSelect__WEBPACK_IMPORTED_MODULE_17__.FormSelect, { id: "add-subnet-type", value: subnetType, onChange: (_e, val) => setSubnetType(val), children: _utils_vpc_utils__WEBPACK_IMPORTED_MODULE_22__.SUBNET_TYPES.map((t) => ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_FormSelect__WEBPACK_IMPORTED_MODULE_17__.FormSelectOption, { value: t, label: t }, t))) }) })] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_15__.ActionGroup, { style: { marginTop: '1rem' }, children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_10__.Button, { variant: "primary", onClick: handleAddSubnet, isLoading: adding, isDisabled: !subnetNameValid || subnetCidr.length === 0 || adding, children: "Add Subnet" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_10__.Button, { variant: "link", onClick: () => { setAddSubnetOpen(false); setAddError(null); }, children: "Cancel" })] })] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Modal__WEBPACK_IMPORTED_MODULE_13__.Modal, { variant: _patternfly_react_core_dist_dynamic_components_Modal__WEBPACK_IMPORTED_MODULE_13__.ModalVariant.small, title: `Remove subnet "${removeTarget}"?`, isOpen: removeTarget !== null, onClose: () => setRemoveTarget(null), children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)("p", { children: ["This will remove the subnet and the controller will delete its namespace (", name, "-", removeTarget, "), UDN, and any RouteAdvertisements."] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_15__.ActionGroup, { style: { marginTop: '1rem' }, children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_10__.Button, { variant: "danger", onClick: handleRemoveSubnet, isLoading: removing, children: "Remove" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_10__.Button, { variant: "link", onClick: () => setRemoveTarget(null), children: "Cancel" })] })] }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Modal__WEBPACK_IMPORTED_MODULE_13__.Modal, { variant: _patternfly_react_core_dist_dynamic_components_Modal__WEBPACK_IMPORTED_MODULE_13__.ModalVariant.small, title: `Delete VPC ${name}?`, isOpen: deleteOpen, onClose: () => setDeleteOpen(false), children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)("p", { children: "This will delete the VPC and all its associated namespaces, UDNs, and RouteAdvertisements." }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Form__WEBPACK_IMPORTED_MODULE_15__.ActionGroup, { style: { marginTop: '1rem' }, children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_10__.Button, { variant: "danger", onClick: handleDelete, isLoading: deleting, children: "Delete" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_10__.Button, { variant: "link", onClick: () => setDeleteOpen(false), children: "Cancel" })] })] })] }));
};
/* harmony default export */ const __WEBPACK_DEFAULT_EXPORT__ = (VPCDetailPage);


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
//# sourceMappingURL=exposed-VPCDetailPage-chunk.js.map