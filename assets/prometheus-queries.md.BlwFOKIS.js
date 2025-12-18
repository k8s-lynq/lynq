import{_ as s,c as a,o as e,a3 as l}from"./chunks/framework.Dt0mZVnE.js";const b=JSON.parse('{"title":"Prometheus Query Examples","description":"","frontmatter":{},"headers":[],"relativePath":"prometheus-queries.md","filePath":"prometheus-queries.md","lastUpdated":1763011429000}'),p={name:"prometheus-queries.md"};function r(i,n,c,o,t,d){return e(),a("div",null,[...n[0]||(n[0]=[l(`<h1 id="prometheus-query-examples" tabindex="-1">Prometheus Query Examples <a class="header-anchor" href="#prometheus-query-examples" aria-label="Permalink to &quot;Prometheus Query Examples&quot;">​</a></h1><p>This document provides ready-to-use PromQL queries for monitoring Lynq.</p><nav class="table-of-contents"><ul><li><a href="#node-health">Node Health</a><ul><li><a href="#check-ready-nodes">Check Ready Nodes</a></li><li><a href="#check-not-ready-nodes">Check Not Ready Nodes</a></li><li><a href="#check-degraded-nodes">Check Degraded Nodes</a></li><li><a href="#resource-health-by-node">Resource Health by Node</a></li></ul></li><li><a href="#conflict-monitoring">Conflict Monitoring</a><ul><li><a href="#current-conflicts">Current Conflicts</a></li><li><a href="#conflict-rate">Conflict Rate</a></li><li><a href="#historical-conflicts">Historical Conflicts</a></li><li><a href="#conflict-policy-analysis">Conflict Policy Analysis</a></li></ul></li><li><a href="#failure-detection">Failure Detection</a><ul><li><a href="#failed-resources">Failed Resources</a></li><li><a href="#failure-trends">Failure Trends</a></li><li><a href="#critical-failures">Critical Failures</a></li></ul></li><li><a href="#performance-monitoring">Performance Monitoring</a><ul><li><a href="#reconciliation-duration">Reconciliation Duration</a></li><li><a href="#reconciliation-rate">Reconciliation Rate</a></li><li><a href="#apply-performance">Apply Performance</a></li></ul></li><li><a href="#hub-health">Hub Health</a><ul><li><a href="#hub-status">Hub Status</a></li><li><a href="#hub-capacity">Hub Capacity</a></li><li><a href="#hub-trends">Hub Trends</a></li></ul></li><li><a href="#capacity-planning">Capacity Planning</a><ul><li><a href="#resource-counts">Resource Counts</a></li><li><a href="#growth-trends">Growth Trends</a></li><li><a href="#load-distribution">Load Distribution</a></li></ul></li><li><a href="#combined-queries">Combined Queries</a><ul><li><a href="#overall-health-dashboard">Overall Health Dashboard</a></li><li><a href="#problem-detection">Problem Detection</a></li><li><a href="#performance-summary">Performance Summary</a></li></ul></li><li><a href="#alert-conditions">Alert Conditions</a><ul><li><a href="#critical-conditions">Critical Conditions</a></li><li><a href="#warning-conditions">Warning Conditions</a></li></ul></li><li><a href="#v1-1-4-enhanced-status-queries">v1.1.4 Enhanced Status Queries</a><ul><li><a href="#new-degraded-reasons">New Degraded Reasons</a></li><li><a href="#ready-condition-granularity">Ready Condition Granularity</a></li><li><a href="#smart-reconciliation-monitoring">Smart Reconciliation Monitoring</a></li><li><a href="#readiness-tracking">Readiness Tracking</a></li></ul></li><li><a href="#tips-for-using-these-queries">Tips for Using These Queries</a><ul><li><a href="#example-filter-by-namespace-and-time">Example: Filter by Namespace and Time</a></li></ul></li><li><a href="#see-also">See Also</a></li></ul></nav><h2 id="node-health" tabindex="-1">Node Health <a class="header-anchor" href="#node-health" aria-label="Permalink to &quot;Node Health&quot;">​</a></h2><h3 id="check-ready-nodes" tabindex="-1">Check Ready Nodes <a class="header-anchor" href="#check-ready-nodes" aria-label="Permalink to &quot;Check Ready Nodes&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># All ready nodes</span></span>
<span class="line"><span>lynqnode_condition_status{type=&quot;Ready&quot;} == 1</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Count ready nodes</span></span>
<span class="line"><span>count(lynqnode_condition_status{type=&quot;Ready&quot;} == 1)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Percentage of ready nodes</span></span>
<span class="line"><span>count(lynqnode_condition_status{type=&quot;Ready&quot;} == 1) / count(lynqnode_condition_status{type=&quot;Ready&quot;}) * 100</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br></div></div><h3 id="check-not-ready-nodes" tabindex="-1">Check Not Ready Nodes <a class="header-anchor" href="#check-not-ready-nodes" aria-label="Permalink to &quot;Check Not Ready Nodes&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># All not ready nodes</span></span>
<span class="line"><span>lynqnode_condition_status{type=&quot;Ready&quot;} != 1</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Count not ready nodes</span></span>
<span class="line"><span>count(lynqnode_condition_status{type=&quot;Ready&quot;} != 1)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># List not ready nodes with details</span></span>
<span class="line"><span>lynqnode_condition_status{type=&quot;Ready&quot;} != 1</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br></div></div><h3 id="check-degraded-nodes" tabindex="-1">Check Degraded Nodes <a class="header-anchor" href="#check-degraded-nodes" aria-label="Permalink to &quot;Check Degraded Nodes&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># All degraded nodes</span></span>
<span class="line"><span>lynqnode_degraded_status == 1</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Count degraded nodes</span></span>
<span class="line"><span>count(lynqnode_degraded_status == 1)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Degraded nodes by reason</span></span>
<span class="line"><span>sum(lynqnode_degraded_status) by (reason)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Top 10 degraded nodes</span></span>
<span class="line"><span>topk(10, lynqnode_degraded_status)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Degraded nodes with resources not ready (v1.1.4+)</span></span>
<span class="line"><span>lynqnode_degraded_status{reason=&quot;ResourcesNotReady&quot;} == 1</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Count by specific degraded reason</span></span>
<span class="line"><span>sum(lynqnode_degraded_status{reason=&quot;ResourcesNotReady&quot;})</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Nodes with resource failures only</span></span>
<span class="line"><span>lynqnode_degraded_status{reason=&quot;ResourceFailures&quot;} == 1</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Nodes with conflicts only</span></span>
<span class="line"><span>lynqnode_degraded_status{reason=&quot;ResourceConflicts&quot;} == 1</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Nodes with both failures and conflicts</span></span>
<span class="line"><span>lynqnode_degraded_status{reason=&quot;ResourceFailuresAndConflicts&quot;} == 1</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br><span class="line-number">12</span><br><span class="line-number">13</span><br><span class="line-number">14</span><br><span class="line-number">15</span><br><span class="line-number">16</span><br><span class="line-number">17</span><br><span class="line-number">18</span><br><span class="line-number">19</span><br><span class="line-number">20</span><br><span class="line-number">21</span><br><span class="line-number">22</span><br><span class="line-number">23</span><br><span class="line-number">24</span><br><span class="line-number">25</span><br><span class="line-number">26</span><br></div></div><h3 id="resource-health-by-node" tabindex="-1">Resource Health by Node <a class="header-anchor" href="#resource-health-by-node" aria-label="Permalink to &quot;Resource Health by Node&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Ready resources per node</span></span>
<span class="line"><span>lynqnode_resources_ready</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Failed resources per node</span></span>
<span class="line"><span>lynqnode_resources_failed</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Resource readiness percentage per node</span></span>
<span class="line"><span>(lynqnode_resources_ready / lynqnode_resources_desired) * 100</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Nodes with 100% resources ready</span></span>
<span class="line"><span>(lynqnode_resources_ready / lynqnode_resources_desired) == 1</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br></div></div><h2 id="conflict-monitoring" tabindex="-1">Conflict Monitoring <a class="header-anchor" href="#conflict-monitoring" aria-label="Permalink to &quot;Conflict Monitoring&quot;">​</a></h2><h3 id="current-conflicts" tabindex="-1">Current Conflicts <a class="header-anchor" href="#current-conflicts" aria-label="Permalink to &quot;Current Conflicts&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Total resources currently in conflict</span></span>
<span class="line"><span>sum(lynqnode_resources_conflicted)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Nodes with conflicts</span></span>
<span class="line"><span>lynqnode_resources_conflicted &gt; 0</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Top 10 nodes with most conflicts</span></span>
<span class="line"><span>topk(10, lynqnode_resources_conflicted)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Conflict percentage per node</span></span>
<span class="line"><span>(lynqnode_resources_conflicted / lynqnode_resources_desired) * 100</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br></div></div><h3 id="conflict-rate" tabindex="-1">Conflict Rate <a class="header-anchor" href="#conflict-rate" aria-label="Permalink to &quot;Conflict Rate&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Conflict rate (conflicts per second)</span></span>
<span class="line"><span>rate(lynqnode_conflicts_total[5m])</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Conflict rate per node</span></span>
<span class="line"><span>sum(rate(lynqnode_conflicts_total[5m])) by (lynqnode)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Conflict rate by resource kind</span></span>
<span class="line"><span>sum(rate(lynqnode_conflicts_total[5m])) by (resource_kind)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Conflict rate by policy</span></span>
<span class="line"><span>sum(rate(lynqnode_conflicts_total[5m])) by (conflict_policy)</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br></div></div><h3 id="historical-conflicts" tabindex="-1">Historical Conflicts <a class="header-anchor" href="#historical-conflicts" aria-label="Permalink to &quot;Historical Conflicts&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Total conflicts in last hour</span></span>
<span class="line"><span>increase(lynqnode_conflicts_total[1h])</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Total conflicts in last 24 hours</span></span>
<span class="line"><span>increase(lynqnode_conflicts_total[24h])</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Conflicts over time (5m windows)</span></span>
<span class="line"><span>sum(increase(lynqnode_conflicts_total[5m])) by (lynqnode)</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br></div></div><h3 id="conflict-policy-analysis" tabindex="-1">Conflict Policy Analysis <a class="header-anchor" href="#conflict-policy-analysis" aria-label="Permalink to &quot;Conflict Policy Analysis&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Conflicts by policy type</span></span>
<span class="line"><span>sum(lynqnode_conflicts_total) by (conflict_policy)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Force policy usage rate</span></span>
<span class="line"><span>rate(lynqnode_conflicts_total{conflict_policy=&quot;Force&quot;}[5m])</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Stuck policy conflicts</span></span>
<span class="line"><span>rate(lynqnode_conflicts_total{conflict_policy=&quot;Stuck&quot;}[5m])</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br></div></div><h2 id="failure-detection" tabindex="-1">Failure Detection <a class="header-anchor" href="#failure-detection" aria-label="Permalink to &quot;Failure Detection&quot;">​</a></h2><h3 id="failed-resources" tabindex="-1">Failed Resources <a class="header-anchor" href="#failed-resources" aria-label="Permalink to &quot;Failed Resources&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Total failed resources</span></span>
<span class="line"><span>sum(lynqnode_resources_failed)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Nodes with failed resources</span></span>
<span class="line"><span>lynqnode_resources_failed &gt; 0</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Top 10 nodes with most failures</span></span>
<span class="line"><span>topk(10, lynqnode_resources_failed)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Failure rate per node</span></span>
<span class="line"><span>(lynqnode_resources_failed / lynqnode_resources_desired) * 100</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br></div></div><h3 id="failure-trends" tabindex="-1">Failure Trends <a class="header-anchor" href="#failure-trends" aria-label="Permalink to &quot;Failure Trends&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Failed resources over time</span></span>
<span class="line"><span>lynqnode_resources_failed</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Increase in failures (last 1h)</span></span>
<span class="line"><span>increase(lynqnode_resources_failed[1h])</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Average failures per node</span></span>
<span class="line"><span>avg(lynqnode_resources_failed)</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br></div></div><h3 id="critical-failures" tabindex="-1">Critical Failures <a class="header-anchor" href="#critical-failures" aria-label="Permalink to &quot;Critical Failures&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Nodes with &gt;50% resources failed</span></span>
<span class="line"><span>(lynqnode_resources_failed / lynqnode_resources_desired) &gt; 0.5</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Nodes with &gt;5 failed resources</span></span>
<span class="line"><span>lynqnode_resources_failed &gt; 5</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Nodes that are both degraded and have failures</span></span>
<span class="line"><span>lynqnode_degraded_status == 1 and lynqnode_resources_failed &gt; 0</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br></div></div><h2 id="performance-monitoring" tabindex="-1">Performance Monitoring <a class="header-anchor" href="#performance-monitoring" aria-label="Permalink to &quot;Performance Monitoring&quot;">​</a></h2><h3 id="reconciliation-duration" tabindex="-1">Reconciliation Duration <a class="header-anchor" href="#reconciliation-duration" aria-label="Permalink to &quot;Reconciliation Duration&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># P50 reconciliation duration</span></span>
<span class="line"><span>histogram_quantile(0.50, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))</span></span>
<span class="line"><span></span></span>
<span class="line"><span># P95 reconciliation duration</span></span>
<span class="line"><span>histogram_quantile(0.95, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))</span></span>
<span class="line"><span></span></span>
<span class="line"><span># P99 reconciliation duration</span></span>
<span class="line"><span>histogram_quantile(0.99, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Max reconciliation duration</span></span>
<span class="line"><span>histogram_quantile(1.0, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br></div></div><h3 id="reconciliation-rate" tabindex="-1">Reconciliation Rate <a class="header-anchor" href="#reconciliation-rate" aria-label="Permalink to &quot;Reconciliation Rate&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Total reconciliation rate</span></span>
<span class="line"><span>rate(lynqnode_reconcile_duration_seconds_count[5m])</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Success rate</span></span>
<span class="line"><span>rate(lynqnode_reconcile_duration_seconds_count{result=&quot;success&quot;}[5m])</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Error rate</span></span>
<span class="line"><span>rate(lynqnode_reconcile_duration_seconds_count{result=&quot;error&quot;}[5m])</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Success percentage</span></span>
<span class="line"><span>(rate(lynqnode_reconcile_duration_seconds_count{result=&quot;success&quot;}[5m]) / rate(lynqnode_reconcile_duration_seconds_count[5m])) * 100</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br></div></div><h3 id="apply-performance" tabindex="-1">Apply Performance <a class="header-anchor" href="#apply-performance" aria-label="Permalink to &quot;Apply Performance&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Apply rate by result</span></span>
<span class="line"><span>sum(rate(apply_attempts_total[5m])) by (result)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Apply rate by resource kind</span></span>
<span class="line"><span>sum(rate(apply_attempts_total[5m])) by (kind)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Apply success rate</span></span>
<span class="line"><span>rate(apply_attempts_total{result=&quot;success&quot;}[5m]) / rate(apply_attempts_total[5m])</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Failed applies by kind</span></span>
<span class="line"><span>sum(rate(apply_attempts_total{result=&quot;error&quot;}[5m])) by (kind)</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br></div></div><h2 id="hub-health" tabindex="-1">Hub Health <a class="header-anchor" href="#hub-health" aria-label="Permalink to &quot;Hub Health&quot;">​</a></h2><h3 id="hub-status" tabindex="-1">Hub Status <a class="header-anchor" href="#hub-status" aria-label="Permalink to &quot;Hub Status&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Desired nodes per hub</span></span>
<span class="line"><span>hub_desired</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Ready nodes per hub</span></span>
<span class="line"><span>hub_ready</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Failed nodes per hub</span></span>
<span class="line"><span>hub_failed</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Hub health percentage</span></span>
<span class="line"><span>(hub_ready / hub_desired) * 100</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br></div></div><h3 id="hub-capacity" tabindex="-1">Hub Capacity <a class="header-anchor" href="#hub-capacity" aria-label="Permalink to &quot;Hub Capacity&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Total desired nodes across all registries</span></span>
<span class="line"><span>sum(hub_desired)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Total ready nodes across all registries</span></span>
<span class="line"><span>sum(hub_ready)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Total failed nodes across all registries</span></span>
<span class="line"><span>sum(hub_failed)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Overall health percentage</span></span>
<span class="line"><span>(sum(hub_ready) / sum(hub_desired)) * 100</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br></div></div><h3 id="hub-trends" tabindex="-1">Hub Trends <a class="header-anchor" href="#hub-trends" aria-label="Permalink to &quot;Hub Trends&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Hub health over time</span></span>
<span class="line"><span>(hub_ready / hub_desired) * 100</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Registries with &gt;90% health</span></span>
<span class="line"><span>(hub_ready / hub_desired) &gt; 0.9</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Unhealthy registries (&lt;80% ready)</span></span>
<span class="line"><span>(hub_ready / hub_desired) &lt; 0.8</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br></div></div><h2 id="capacity-planning" tabindex="-1">Capacity Planning <a class="header-anchor" href="#capacity-planning" aria-label="Permalink to &quot;Capacity Planning&quot;">​</a></h2><h3 id="resource-counts" tabindex="-1">Resource Counts <a class="header-anchor" href="#resource-counts" aria-label="Permalink to &quot;Resource Counts&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Total desired resources across all nodes</span></span>
<span class="line"><span>sum(lynqnode_resources_desired)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Total ready resources</span></span>
<span class="line"><span>sum(lynqnode_resources_ready)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Total failed resources</span></span>
<span class="line"><span>sum(lynqnode_resources_failed)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Total conflicted resources</span></span>
<span class="line"><span>sum(lynqnode_resources_conflicted)</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br></div></div><h3 id="growth-trends" tabindex="-1">Growth Trends <a class="header-anchor" href="#growth-trends" aria-label="Permalink to &quot;Growth Trends&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Desired node growth rate</span></span>
<span class="line"><span>rate(hub_desired[24h])</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Resource growth per node</span></span>
<span class="line"><span>rate(lynqnode_resources_desired[24h])</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Average resources per node</span></span>
<span class="line"><span>avg(lynqnode_resources_desired)</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br></div></div><h3 id="load-distribution" tabindex="-1">Load Distribution <a class="header-anchor" href="#load-distribution" aria-label="Permalink to &quot;Load Distribution&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Top 10 nodes by resource count</span></span>
<span class="line"><span>topk(10, lynqnode_resources_desired)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Bottom 10 nodes by resource count</span></span>
<span class="line"><span>bottomk(10, lynqnode_resources_desired)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Nodes with &gt;100 resources</span></span>
<span class="line"><span>lynqnode_resources_desired &gt; 100</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Distribution of resources per node</span></span>
<span class="line"><span>histogram_quantile(0.50, lynqnode_resources_desired)</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br></div></div><h2 id="combined-queries" tabindex="-1">Combined Queries <a class="header-anchor" href="#combined-queries" aria-label="Permalink to &quot;Combined Queries&quot;">​</a></h2><h3 id="overall-health-dashboard" tabindex="-1">Overall Health Dashboard <a class="header-anchor" href="#overall-health-dashboard" aria-label="Permalink to &quot;Overall Health Dashboard&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Total nodes</span></span>
<span class="line"><span>count(lynqnode_condition_status{type=&quot;Ready&quot;})</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Ready percentage</span></span>
<span class="line"><span>count(lynqnode_condition_status{type=&quot;Ready&quot;} == 1) / count(lynqnode_condition_status{type=&quot;Ready&quot;}) * 100</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Total resources</span></span>
<span class="line"><span>sum(lynqnode_resources_desired)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Ready resources percentage</span></span>
<span class="line"><span>sum(lynqnode_resources_ready) / sum(lynqnode_resources_desired) * 100</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Active conflicts</span></span>
<span class="line"><span>sum(lynqnode_resources_conflicted)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Total failures</span></span>
<span class="line"><span>sum(lynqnode_resources_failed)</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br><span class="line-number">12</span><br><span class="line-number">13</span><br><span class="line-number">14</span><br><span class="line-number">15</span><br><span class="line-number">16</span><br><span class="line-number">17</span><br></div></div><h3 id="problem-detection" tabindex="-1">Problem Detection <a class="header-anchor" href="#problem-detection" aria-label="Permalink to &quot;Problem Detection&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Nodes with issues (not ready OR degraded OR conflicts OR failures)</span></span>
<span class="line"><span>(lynqnode_condition_status{type=&quot;Ready&quot;} != 1)</span></span>
<span class="line"><span>or (lynqnode_degraded_status == 1)</span></span>
<span class="line"><span>or (lynqnode_resources_conflicted &gt; 0)</span></span>
<span class="line"><span>or (lynqnode_resources_failed &gt; 0)</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Count problematic nodes</span></span>
<span class="line"><span>count(</span></span>
<span class="line"><span>  (lynqnode_condition_status{type=&quot;Ready&quot;} != 1)</span></span>
<span class="line"><span>  or (lynqnode_degraded_status == 1)</span></span>
<span class="line"><span>  or (lynqnode_resources_conflicted &gt; 0)</span></span>
<span class="line"><span>  or (lynqnode_resources_failed &gt; 0)</span></span>
<span class="line"><span>)</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br><span class="line-number">12</span><br><span class="line-number">13</span><br></div></div><h3 id="performance-summary" tabindex="-1">Performance Summary <a class="header-anchor" href="#performance-summary" aria-label="Permalink to &quot;Performance Summary&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># P95 latency, error rate, and throughput</span></span>
<span class="line"><span>{</span></span>
<span class="line"><span>  p95_latency: histogram_quantile(0.95, rate(lynqnode_reconcile_duration_seconds_bucket[5m])),</span></span>
<span class="line"><span>  error_rate: rate(lynqnode_reconcile_duration_seconds_count{result=&quot;error&quot;}[5m]),</span></span>
<span class="line"><span>  throughput: rate(lynqnode_reconcile_duration_seconds_count[5m])</span></span>
<span class="line"><span>}</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br></div></div><h2 id="alert-conditions" tabindex="-1">Alert Conditions <a class="header-anchor" href="#alert-conditions" aria-label="Permalink to &quot;Alert Conditions&quot;">​</a></h2><p>These queries are used in the alert rules (<code>config/prometheus/alerts.yaml</code>):</p><h3 id="critical-conditions" tabindex="-1">Critical Conditions <a class="header-anchor" href="#critical-conditions" aria-label="Permalink to &quot;Critical Conditions&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Node has failed resources</span></span>
<span class="line"><span>lynqnode_resources_failed &gt; 0</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Node is degraded</span></span>
<span class="line"><span>lynqnode_degraded_status &gt; 0</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Node not ready</span></span>
<span class="line"><span>lynqnode_condition_status{type=&quot;Ready&quot;} != 1</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Hub has many failures</span></span>
<span class="line"><span>hub_failed &gt; 5 or (hub_failed / hub_desired &gt; 0.5 and hub_desired &gt; 0)</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br></div></div><h3 id="warning-conditions" tabindex="-1">Warning Conditions <a class="header-anchor" href="#warning-conditions" aria-label="Permalink to &quot;Warning Conditions&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Resources in conflict</span></span>
<span class="line"><span>lynqnode_resources_conflicted &gt; 0</span></span>
<span class="line"><span></span></span>
<span class="line"><span># High conflict rate</span></span>
<span class="line"><span>rate(lynqnode_conflicts_total[5m]) &gt; 0.1</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Resources mismatch</span></span>
<span class="line"><span>lynqnode_resources_ready != lynqnode_resources_desired and lynqnode_resources_desired &gt; 0</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Slow reconciliation</span></span>
<span class="line"><span>histogram_quantile(0.95, rate(lynqnode_reconcile_duration_seconds_bucket[5m])) &gt; 30</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Resources not ready (v1.1.4+)</span></span>
<span class="line"><span>lynqnode_degraded_status{reason=&quot;ResourcesNotReady&quot;} &gt; 0</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Nodes with both failures and conflicts (v1.1.4+)</span></span>
<span class="line"><span>lynqnode_degraded_status{reason=&quot;ResourceFailuresAndConflicts&quot;} &gt; 0</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br><span class="line-number">12</span><br><span class="line-number">13</span><br><span class="line-number">14</span><br><span class="line-number">15</span><br><span class="line-number">16</span><br><span class="line-number">17</span><br></div></div><h2 id="v1-1-4-enhanced-status-queries" tabindex="-1">v1.1.4 Enhanced Status Queries <a class="header-anchor" href="#v1-1-4-enhanced-status-queries" aria-label="Permalink to &quot;v1.1.4 Enhanced Status Queries&quot;">​</a></h2><div class="tip custom-block"><p class="custom-block-title">New in v1.1.4</p><p>v1.1.4 introduces more granular degraded condition reasons and smart reconciliation with 30-second requeue interval.</p></div><h3 id="new-degraded-reasons" tabindex="-1">New Degraded Reasons <a class="header-anchor" href="#new-degraded-reasons" aria-label="Permalink to &quot;New Degraded Reasons&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># All nodes degraded due to resources not ready</span></span>
<span class="line"><span>lynqnode_degraded_status{reason=&quot;ResourcesNotReady&quot;} == 1</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Nodes with resource failures only</span></span>
<span class="line"><span>lynqnode_degraded_status{reason=&quot;ResourceFailures&quot;} == 1</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Nodes with conflicts only</span></span>
<span class="line"><span>lynqnode_degraded_status{reason=&quot;ResourceConflicts&quot;} == 1</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Nodes with both failures and conflicts</span></span>
<span class="line"><span>lynqnode_degraded_status{reason=&quot;ResourceFailuresAndConflicts&quot;} == 1</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Count nodes by degraded reason</span></span>
<span class="line"><span>sum(lynqnode_degraded_status == 1) by (reason)</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br><span class="line-number">12</span><br><span class="line-number">13</span><br><span class="line-number">14</span><br></div></div><h3 id="ready-condition-granularity" tabindex="-1">Ready Condition Granularity <a class="header-anchor" href="#ready-condition-granularity" aria-label="Permalink to &quot;Ready Condition Granularity&quot;">​</a></h3><p>Check why nodes are not ready:</p><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># All not-ready nodes with detailed reason</span></span>
<span class="line"><span>lynqnode_condition_status{type=&quot;Ready&quot;} != 1</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Degraded nodes with Ready=False</span></span>
<span class="line"><span>lynqnode_condition_status{type=&quot;Ready&quot;} != 1 and lynqnode_condition_status{type=&quot;Degraded&quot;} == 1</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Query node annotations for reason details (requires external tooling)</span></span>
<span class="line"><span># Reasons: ResourcesFailedAndConflicted, ResourcesConflicted,</span></span>
<span class="line"><span>#          ResourcesFailed, NotAllResourcesReady</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br></div></div><h3 id="smart-reconciliation-monitoring" tabindex="-1">Smart Reconciliation Monitoring <a class="header-anchor" href="#smart-reconciliation-monitoring" aria-label="Permalink to &quot;Smart Reconciliation Monitoring&quot;">​</a></h3><p>Monitor the 30-second requeue behavior:</p><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Reconciliation frequency (should show ~2 per minute per node in v1.1.4+)</span></span>
<span class="line"><span>rate(lynqnode_reconcile_duration_seconds_count[5m])</span></span>
<span class="line"><span></span></span>
<span class="line"><span># P50 latency (should remain low due to fast requeue)</span></span>
<span class="line"><span>histogram_quantile(0.50, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))</span></span>
<span class="line"><span></span></span>
<span class="line"><span># P95 latency (watch for spikes &gt; 30s)</span></span>
<span class="line"><span>histogram_quantile(0.95, rate(lynqnode_reconcile_duration_seconds_bucket[5m]))</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Detect reconciliation bottlenecks</span></span>
<span class="line"><span>histogram_quantile(0.95, rate(lynqnode_reconcile_duration_seconds_bucket[5m])) &gt; 10</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Status-only reconciliations (fast path)</span></span>
<span class="line"><span>rate(lynqnode_reconcile_duration_seconds_count{result=&quot;status_only&quot;}[5m])</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br><span class="line-number">10</span><br><span class="line-number">11</span><br><span class="line-number">12</span><br><span class="line-number">13</span><br><span class="line-number">14</span><br></div></div><h3 id="readiness-tracking" tabindex="-1">Readiness Tracking <a class="header-anchor" href="#readiness-tracking" aria-label="Permalink to &quot;Readiness Tracking&quot;">​</a></h3><p>Track how quickly resources become ready:</p><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Percentage of resources ready</span></span>
<span class="line"><span>(sum(lynqnode_resources_ready) / sum(lynqnode_resources_desired)) * 100</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Nodes with incomplete readiness</span></span>
<span class="line"><span>lynqnode_resources_ready &lt; lynqnode_resources_desired</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Average time to readiness (approximation via reconciliation duration)</span></span>
<span class="line"><span>avg(rate(lynqnode_reconcile_duration_seconds_sum{result=&quot;success&quot;}[5m]) /</span></span>
<span class="line"><span>    rate(lynqnode_reconcile_duration_seconds_count{result=&quot;success&quot;}[5m]))</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br><span class="line-number">6</span><br><span class="line-number">7</span><br><span class="line-number">8</span><br><span class="line-number">9</span><br></div></div><h2 id="tips-for-using-these-queries" tabindex="-1">Tips for Using These Queries <a class="header-anchor" href="#tips-for-using-these-queries" aria-label="Permalink to &quot;Tips for Using These Queries&quot;">​</a></h2><ol><li><strong>Adjust Time Windows</strong>: Change <code>[5m]</code>, <code>[1h]</code>, <code>[24h]</code> based on your needs</li><li><strong>Filter by Namespace</strong>: Add <code>{namespace=&quot;default&quot;}</code> to filter</li><li><strong>Filter by Node</strong>: Add <code>{lynqnode=&quot;my-node&quot;}</code> to focus on specific nodes</li><li><strong>Combine Queries</strong>: Use <code>and</code>, <code>or</code>, <code>unless</code> for complex conditions</li><li><strong>Aggregation</strong>: Use <code>sum</code>, <code>avg</code>, <code>max</code>, <code>min</code> for aggregations</li><li><strong>Top/Bottom N</strong>: Use <code>topk(N, ...)</code> or <code>bottomk(N, ...)</code></li></ol><h3 id="example-filter-by-namespace-and-time" tabindex="-1">Example: Filter by Namespace and Time <a class="header-anchor" href="#example-filter-by-namespace-and-time" aria-label="Permalink to &quot;Example: Filter by Namespace and Time&quot;">​</a></h3><div class="language-promql line-numbers-mode"><button title="Copy Code" class="copy"></button><span class="lang">promql</span><pre class="shiki github-dark vp-code" tabindex="0"><code><span class="line"><span># Failed resources in default namespace, last 1 hour</span></span>
<span class="line"><span>lynqnode_resources_failed{namespace=&quot;default&quot;}[1h]</span></span>
<span class="line"><span></span></span>
<span class="line"><span># Conflicts for specific node in last 5 minutes</span></span>
<span class="line"><span>rate(lynqnode_conflicts_total{lynqnode=&quot;acme-prod-template&quot;, namespace=&quot;default&quot;}[5m])</span></span></code></pre><div class="line-numbers-wrapper" aria-hidden="true"><span class="line-number">1</span><br><span class="line-number">2</span><br><span class="line-number">3</span><br><span class="line-number">4</span><br><span class="line-number">5</span><br></div></div><h2 id="see-also" tabindex="-1">See Also <a class="header-anchor" href="#see-also" aria-label="Permalink to &quot;See Also&quot;">​</a></h2><ul><li><a href="./monitoring.html">Monitoring Guide</a> - Complete monitoring documentation</li><li><a href="../config/prometheus/alerts.yaml">Alert Rules</a> - Prometheus alert rules</li><li><a href="./troubleshooting.html">Troubleshooting</a> - Common issues and solutions</li></ul>`,81)])])}const m=s(p,[["render",r]]);export{b as __pageData,m as default};
