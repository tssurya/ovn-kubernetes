"use strict";
(self["webpackChunkovn_vpc_console_plugin"] = self["webpackChunkovn_vpc_console_plugin"] || []).push([["exposed-VPCListPage"],{

/***/ "./components/VPCListPage.tsx"
/*!************************************!*\
  !*** ./components/VPCListPage.tsx ***!
  \************************************/
(__unused_webpack_module, __webpack_exports__, __webpack_require__) {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   "default": () => (__WEBPACK_DEFAULT_EXPORT__)
/* harmony export */ });
/* harmony import */ var react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(/*! react/jsx-runtime */ "../node_modules/react/jsx-runtime.js");
/* harmony import */ var react_router_dom_v5_compat__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__(/*! react-router-dom-v5-compat */ "webpack/sharing/consume/default/react-router-dom-v5-compat");
/* harmony import */ var react_router_dom_v5_compat__WEBPACK_IMPORTED_MODULE_1___default = /*#__PURE__*/__webpack_require__.n(react_router_dom_v5_compat__WEBPACK_IMPORTED_MODULE_1__);
/* harmony import */ var _openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__(/*! @openshift-console/dynamic-plugin-sdk */ "webpack/sharing/consume/default/@openshift-console/dynamic-plugin-sdk");
/* harmony import */ var _openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_2___default = /*#__PURE__*/__webpack_require__.n(_openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_2__);
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_3__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Page */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Page/@patternfly/react-core/dist/dynamic/components/Page");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Title__WEBPACK_IMPORTED_MODULE_4__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Title */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Title/@patternfly/react-core/dist/dynamic/components/Title");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_5__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Button */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Button/@patternfly/react-core/dist/dynamic/components/Button");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Toolbar__WEBPACK_IMPORTED_MODULE_6__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Toolbar */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Toolbar/@patternfly/react-core/dist/dynamic/components/Toolbar");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_7__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/EmptyState */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/EmptyState/@patternfly/react-core/dist/dynamic/components/EmptyState");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Spinner__WEBPACK_IMPORTED_MODULE_8__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Spinner */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Spinner/@patternfly/react-core/dist/dynamic/components/Spinner");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Label__WEBPACK_IMPORTED_MODULE_9__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Label */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Label/@patternfly/react-core/dist/dynamic/components/Label");
/* harmony import */ var _patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_10__ = __webpack_require__(/*! @patternfly/react-table/dist/dynamic/components/Table */ "webpack/sharing/consume/default/@patternfly/react-table/dist/dynamic/components/Table/@patternfly/react-table/dist/dynamic/components/Table");
/* harmony import */ var _models_vpc__WEBPACK_IMPORTED_MODULE_11__ = __webpack_require__(/*! ../models/vpc */ "./models/vpc.ts");
/* harmony import */ var _utils_vpc_utils__WEBPACK_IMPORTED_MODULE_12__ = __webpack_require__(/*! ../utils/vpc-utils */ "./utils/vpc-utils.ts");






















const VPCListPage = () => {
    const navigate = (0,react_router_dom_v5_compat__WEBPACK_IMPORTED_MODULE_1__.useNavigate)();
    const [vpcs, loaded, loadError] = (0,_openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_2__.useK8sWatchResource)({
        groupVersionKind: _models_vpc__WEBPACK_IMPORTED_MODULE_11__.vpcGroupVersionKind,
        isList: true,
    });
    const vpcList = vpcs || [];
    if (loadError) {
        return ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_3__.Page, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_3__.PageSection, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_7__.EmptyState, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_7__.EmptyStateBody, { children: ["Error loading VPCs: ", loadError.message || String(loadError)] }) }) }) }));
    }
    if (!loaded) {
        return ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_3__.Page, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_3__.PageSection, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Spinner__WEBPACK_IMPORTED_MODULE_8__.Spinner, { size: "xl" }) }) }));
    }
    return ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_3__.Page, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_3__.PageSection, { variant: "default", children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Toolbar__WEBPACK_IMPORTED_MODULE_6__.Toolbar, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Toolbar__WEBPACK_IMPORTED_MODULE_6__.ToolbarContent, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Toolbar__WEBPACK_IMPORTED_MODULE_6__.ToolbarItem, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Title__WEBPACK_IMPORTED_MODULE_4__.Title, { headingLevel: "h1", children: "Virtual Private Clouds" }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Toolbar__WEBPACK_IMPORTED_MODULE_6__.ToolbarItem, { align: { default: 'alignEnd' }, children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_5__.Button, { variant: "primary", onClick: () => navigate('/k8s/cluster/vpcs/~new'), children: "Create VPC" }) })] }) }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_3__.PageSection, { children: vpcList.length === 0 ? ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_7__.EmptyState, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Title__WEBPACK_IMPORTED_MODULE_4__.Title, { headingLevel: "h2", size: "lg", children: "No VPCs found" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_7__.EmptyStateBody, { children: "Create a VPC to define isolated virtual networks with subnets." }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Button__WEBPACK_IMPORTED_MODULE_5__.Button, { variant: "primary", onClick: () => navigate('/k8s/cluster/vpcs/~new'), children: "Create VPC" })] })) : ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_10__.Table, { "aria-label": "VPCs", variant: "compact", children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_10__.Thead, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_10__.Tr, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_10__.Th, { children: "Name" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_10__.Th, { children: "Subnets" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_10__.Th, { children: "Status" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_10__.Th, { children: "Age" })] }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_10__.Tbody, { children: vpcList.map((vpc) => ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_10__.Tr, { isClickable: true, onRowClick: () => navigate(`/k8s/cluster/vpcs/${vpc.metadata?.name}`), children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_10__.Td, { dataLabel: "Name", children: vpc.metadata?.name }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_10__.Td, { dataLabel: "Subnets", children: vpc.spec?.subnets?.length ?? 0 }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_10__.Td, { dataLabel: "Status", children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Label__WEBPACK_IMPORTED_MODULE_9__.Label, { color: (0,_utils_vpc_utils__WEBPACK_IMPORTED_MODULE_12__.isVPCReady)(vpc) ? 'green' : 'orange', children: (0,_utils_vpc_utils__WEBPACK_IMPORTED_MODULE_12__.getVPCStatusLabel)(vpc) }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_10__.Td, { dataLabel: "Age", children: (0,_utils_vpc_utils__WEBPACK_IMPORTED_MODULE_12__.timeAgo)(vpc.metadata?.creationTimestamp) })] }, vpc.metadata?.name))) })] })) })] }));
};
/* harmony default export */ const __WEBPACK_DEFAULT_EXPORT__ = (VPCListPage);


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
//# sourceMappingURL=exposed-VPCListPage-chunk.js.map