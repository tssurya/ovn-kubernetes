"use strict";
(self["webpackChunkovn_vpc_console_plugin"] = self["webpackChunkovn_vpc_console_plugin"] || []).push([["exposed-UDNListPage"],{

/***/ "./components/UDNListPage.tsx"
/*!************************************!*\
  !*** ./components/UDNListPage.tsx ***!
  \************************************/
(__unused_webpack_module, __webpack_exports__, __webpack_require__) {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   "default": () => (__WEBPACK_DEFAULT_EXPORT__)
/* harmony export */ });
/* harmony import */ var react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__(/*! react/jsx-runtime */ "../node_modules/react/jsx-runtime.js");
/* harmony import */ var _openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__(/*! @openshift-console/dynamic-plugin-sdk */ "webpack/sharing/consume/default/@openshift-console/dynamic-plugin-sdk");
/* harmony import */ var _openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_1___default = /*#__PURE__*/__webpack_require__.n(_openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_1__);
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Page */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Page/@patternfly/react-core/dist/dynamic/components/Page");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Title__WEBPACK_IMPORTED_MODULE_3__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Title */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Title/@patternfly/react-core/dist/dynamic/components/Title");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Spinner__WEBPACK_IMPORTED_MODULE_4__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Spinner */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Spinner/@patternfly/react-core/dist/dynamic/components/Spinner");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_5__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/EmptyState */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/EmptyState/@patternfly/react-core/dist/dynamic/components/EmptyState");
/* harmony import */ var _patternfly_react_core_dist_dynamic_components_Label__WEBPACK_IMPORTED_MODULE_6__ = __webpack_require__(/*! @patternfly/react-core/dist/dynamic/components/Label */ "webpack/sharing/consume/default/@patternfly/react-core/dist/dynamic/components/Label/@patternfly/react-core/dist/dynamic/components/Label");
/* harmony import */ var _patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__ = __webpack_require__(/*! @patternfly/react-table/dist/dynamic/components/Table */ "webpack/sharing/consume/default/@patternfly/react-table/dist/dynamic/components/Table/@patternfly/react-table/dist/dynamic/components/Table");
/* harmony import */ var _models_udn__WEBPACK_IMPORTED_MODULE_8__ = __webpack_require__(/*! ../models/udn */ "./models/udn.ts");
/* harmony import */ var _utils_vpc_utils__WEBPACK_IMPORTED_MODULE_9__ = __webpack_require__(/*! ../utils/vpc-utils */ "./utils/vpc-utils.ts");

















function getUDNStatus(udn) {
    const ready = udn.status?.conditions?.find((c) => c.type === 'NetworkReady');
    if (!ready)
        return { label: 'Pending', color: 'grey' };
    return ready.status === 'True'
        ? { label: 'Ready', color: 'green' }
        : { label: ready.reason || 'Not Ready', color: 'orange' };
}
function getSubnets(udn) {
    if (udn.spec?.layer2?.subnets)
        return udn.spec.layer2.subnets.join(', ');
    if (udn.spec?.layer3?.subnets)
        return udn.spec.layer3.subnets.map((s) => s.cidr).join(', ');
    return '-';
}
const UDNListPage = () => {
    const [udns, loaded, loadError] = (0,_openshift_console_dynamic_plugin_sdk__WEBPACK_IMPORTED_MODULE_1__.useK8sWatchResource)({
        groupVersionKind: _models_udn__WEBPACK_IMPORTED_MODULE_8__.udnGroupVersionKind,
        isList: true,
        namespaced: true,
    });
    const udnList = udns || [];
    if (loadError) {
        return ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_2__.Page, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_2__.PageSection, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_5__.EmptyState, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_5__.EmptyStateBody, { children: ["Error loading UDNs: ", loadError.message || String(loadError)] }) }) }) }));
    }
    if (!loaded) {
        return ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_2__.Page, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_2__.PageSection, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Spinner__WEBPACK_IMPORTED_MODULE_4__.Spinner, { size: "xl" }) }) }));
    }
    return ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_2__.Page, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_2__.PageSection, { variant: "default", children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Title__WEBPACK_IMPORTED_MODULE_3__.Title, { headingLevel: "h1", children: "UserDefinedNetworks" }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Page__WEBPACK_IMPORTED_MODULE_2__.PageSection, { children: udnList.length === 0 ? ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_5__.EmptyState, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Title__WEBPACK_IMPORTED_MODULE_3__.Title, { headingLevel: "h2", size: "lg", children: "No UserDefinedNetworks found" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_EmptyState__WEBPACK_IMPORTED_MODULE_5__.EmptyStateBody, { children: "UDNs are created automatically by the VPC controller for each VPC subnet." })] })) : ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Table, { "aria-label": "UserDefinedNetworks", variant: "compact", children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Thead, { children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Tr, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Th, { children: "Name" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Th, { children: "Namespace" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Th, { children: "Topology" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Th, { children: "Role" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Th, { children: "Subnets" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Th, { children: "Transport" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Th, { children: "Status" }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Th, { children: "Age" })] }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Tbody, { children: udnList.map((udn) => {
                                const status = getUDNStatus(udn);
                                const role = udn.spec?.layer2?.role || udn.spec?.layer3?.role || '-';
                                return ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsxs)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Tr, { children: [(0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Td, { dataLabel: "Name", children: udn.metadata?.name }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Td, { dataLabel: "Namespace", children: udn.metadata?.namespace }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Td, { dataLabel: "Topology", children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Label__WEBPACK_IMPORTED_MODULE_6__.Label, { color: "blue", children: udn.spec?.topology || '-' }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Td, { dataLabel: "Role", children: role }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Td, { dataLabel: "Subnets", children: getSubnets(udn) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Td, { dataLabel: "Transport", children: udn.spec?.transport ? ((0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Label__WEBPACK_IMPORTED_MODULE_6__.Label, { color: "purple", children: udn.spec.transport })) : '-' }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Td, { dataLabel: "Status", children: (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_core_dist_dynamic_components_Label__WEBPACK_IMPORTED_MODULE_6__.Label, { color: status.color, children: status.label }) }), (0,react_jsx_runtime__WEBPACK_IMPORTED_MODULE_0__.jsx)(_patternfly_react_table_dist_dynamic_components_Table__WEBPACK_IMPORTED_MODULE_7__.Td, { dataLabel: "Age", children: (0,_utils_vpc_utils__WEBPACK_IMPORTED_MODULE_9__.timeAgo)(udn.metadata?.creationTimestamp) })] }, `${udn.metadata?.namespace}/${udn.metadata?.name}`));
                            }) })] })) })] }));
};
/* harmony default export */ const __WEBPACK_DEFAULT_EXPORT__ = (UDNListPage);


/***/ },

/***/ "./models/udn.ts"
/*!***********************!*\
  !*** ./models/udn.ts ***!
  \***********************/
(__unused_webpack_module, __webpack_exports__, __webpack_require__) {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   UDNModel: () => (/* binding */ UDNModel),
/* harmony export */   udnGroupVersionKind: () => (/* binding */ udnGroupVersionKind)
/* harmony export */ });
const UDNModel = {
    apiGroup: 'k8s.ovn.org',
    apiVersion: 'v1',
    kind: 'UserDefinedNetwork',
    plural: 'userdefinednetworks',
    label: 'UserDefinedNetwork',
    labelPlural: 'UserDefinedNetworks',
    abbr: 'UDN',
    namespaced: true,
};
const udnGroupVersionKind = {
    group: 'k8s.ovn.org',
    version: 'v1',
    kind: 'UserDefinedNetwork',
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
//# sourceMappingURL=exposed-UDNListPage-chunk.js.map