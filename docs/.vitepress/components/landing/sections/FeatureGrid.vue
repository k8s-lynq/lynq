<template>
  <section class="feature-grid-section bg-lynq-bg">
    <div class="fg-inner mx-auto">
      <SectionHeader
        label="Capabilities"
        title="Everything each row needs, handled"
        subtitle="One database row in, a full set of reconciled Kubernetes resources out — with the controls to keep it safe."
        accent="purple"
      />

      <div class="fg-grid mt-14 grid gap-3">
        <!-- 1. Reads your database (a DB table being polled row by row) -->
        <article class="fcard span-3 flex flex-col rounded-lynq border border-lynq-border bg-lynq-card p-6">
          <div class="fviz">
            <div class="dbwin">
              <div class="db-title">
                <svg class="db-ico" viewBox="0 0 24 24" aria-hidden="true">
                  <ellipse cx="12" cy="5" rx="8" ry="3" />
                  <path d="M4 5v14c0 1.66 3.58 3 8 3s8-1.34 8-3V5" />
                  <path d="M4 12c0 1.66 3.58 3 8 3s8-1.34 8-3" />
                </svg>
                <span class="db-name">node_configs</span>
                <span class="db-caret" aria-hidden="true"></span>
                <span class="db-tag">MySQL</span>
              </div>
              <div class="db-tbl">
                <div class="db-r db-h">
                  <span>node_id</span><span class="db-flag">is_active</span>
                </div>
                <div class="db-r">
                  <span>acme-corp</span><span class="db-flag on">1</span>
                  <span class="db-emit em1" aria-hidden="true"></span>
                </div>
                <div class="db-r">
                  <span>beta-inc</span><span class="db-flag on">1</span>
                  <span class="db-emit em2" aria-hidden="true"></span>
                </div>
                <div class="db-r">
                  <span>gamma-llc</span><span class="db-flag off">0</span>
                </div>
                <span class="db-scan" aria-hidden="true"></span>
              </div>
            </div>
          </div>
          <h3 class="ftitle font-medium text-lynq-text">Reads your database</h3>
          <p class="fdesc text-lynq-dim">
            Point Lynq at a MySQL table. Every row where <code>activate</code> is
            true becomes a managed node — no pipelines, no glue code.
          </p>
        </article>

        <!-- 2. Server-Side Apply (scan beam over fields) -->
        <article class="fcard span-3 flex flex-col rounded-lynq border border-lynq-border bg-lynq-card p-6">
          <div class="fviz">
            <!-- SSA applies the manifest field by field under fieldManager: lynq.
                 An apply sweep stamps each lynq-owned field (teal) in turn, while
                 the field another manager (helm) owns is left untouched. -->
            <div class="ssa" aria-hidden="true">
              <div class="ssa-doc">
                <div class="ssa-bar">
                  <span class="ssa-file">acme-corp-app · Deployment</span>
                  <span class="ssa-apply">SSA apply</span>
                </div>
                <div class="ssa-row sr1">
                  <span class="ssa-k">spec.replicas</span>
                  <span class="ssa-v">3</span>
                  <span class="ssa-own o-lynq">lynq</span>
                </div>
                <div class="ssa-row sr2">
                  <span class="ssa-k">spec.template…image</span>
                  <span class="ssa-v">nginx:1.27</span>
                  <span class="ssa-own o-lynq">lynq</span>
                </div>
                <div class="ssa-row sr3">
                  <span class="ssa-k">spec.resources.limits</span>
                  <span class="ssa-v">512Mi</span>
                  <span class="ssa-own o-lynq">lynq</span>
                </div>
                <div class="ssa-row sr-keep">
                  <span class="ssa-k">metadata.annotations</span>
                  <span class="ssa-v">kept</span>
                  <span class="ssa-own o-other">helm</span>
                </div>
                <span class="ssa-scan"></span>
              </div>
              <div class="ssa-fm">
                <span class="fm-dot"></span>
                <span><b>fieldManager: lynq</b> owns its fields · <em>helm</em>'s field untouched</span>
              </div>
            </div>
          </div>
          <h3 class="ftitle font-medium text-lynq-text">Server-Side Apply</h3>
          <p class="fdesc text-lynq-dim">
            Resources are applied with SSA under the <code>lynq</code> field
            manager — Lynq owns exactly its fields and never clobbers the rest.
          </p>
        </article>

        <!-- 3. Dependency-aware ordering (node graph + traveling pulse) -->
        <article class="fcard span-2 flex flex-col rounded-lynq border border-lynq-border bg-lynq-card p-6">
          <div class="fviz">
            <!-- A real dependency DAG (1 → 3 → 2 → 1). Namespace applies first;
                 the three resources that depend on it then apply IN PARALLEL;
                 the two workloads wait for those, and Ingress waits for both.
                 Nodes fill level by level (parallel within a level), edges draw
                 in as each dependency clears — pending edges stay dashed. -->
            <div class="dep" aria-hidden="true">
              <svg class="dep-svg" viewBox="0 0 290 200" preserveAspectRatio="xMidYMid meet">
                <!-- pending edges: faint dashed connectors -->
                <g fill="none" stroke="rgba(255,255,255,0.1)" stroke-width="1.3" stroke-dasharray="3 3">
                  <line x1="34" y1="100" x2="114" y2="50" /><line x1="34" y1="100" x2="114" y2="100" /><line x1="34" y1="100" x2="114" y2="150" />
                  <line x1="114" y1="50" x2="196" y2="74" /><line x1="114" y1="100" x2="196" y2="74" /><line x1="114" y1="100" x2="196" y2="126" /><line x1="114" y1="150" x2="196" y2="126" />
                  <line x1="196" y1="74" x2="262" y2="100" /><line x1="196" y1="126" x2="262" y2="100" />
                </g>
                <!-- completed edges: solid teal, drawn in level by level -->
                <g fill="none" stroke="#33aca8" stroke-width="1.6" stroke-linecap="round">
                  <line class="ed e-l1" pathLength="100" x1="34" y1="100" x2="114" y2="50" /><line class="ed e-l1" pathLength="100" x1="34" y1="100" x2="114" y2="100" /><line class="ed e-l1" pathLength="100" x1="34" y1="100" x2="114" y2="150" />
                  <line class="ed e-l2" pathLength="100" x1="114" y1="50" x2="196" y2="74" /><line class="ed e-l2" pathLength="100" x1="114" y1="100" x2="196" y2="74" /><line class="ed e-l2" pathLength="100" x1="114" y1="100" x2="196" y2="126" /><line class="ed e-l2" pathLength="100" x1="114" y1="150" x2="196" y2="126" />
                  <line class="ed e-l3" pathLength="100" x1="196" y1="74" x2="262" y2="100" /><line class="ed e-l3" pathLength="100" x1="196" y1="126" x2="262" y2="100" />
                </g>
                <!-- nodes: hollow ring + fill core, lit level by level -->
                <g class="dep-node lvl0"><circle class="ring" cx="34" cy="100" r="9" /><circle class="core" cx="34" cy="100" r="5" /></g>
                <g class="dep-node lvl1"><circle class="ring" cx="114" cy="50" r="9" /><circle class="core" cx="114" cy="50" r="5" /></g>
                <g class="dep-node lvl1"><circle class="ring" cx="114" cy="100" r="9" /><circle class="core" cx="114" cy="100" r="5" /></g>
                <g class="dep-node lvl1"><circle class="ring" cx="114" cy="150" r="9" /><circle class="core" cx="114" cy="150" r="5" /></g>
                <g class="dep-node lvl2"><circle class="ring" cx="196" cy="74" r="9" /><circle class="core" cx="196" cy="74" r="5" /></g>
                <g class="dep-node lvl2"><circle class="ring" cx="196" cy="126" r="9" /><circle class="core" cx="196" cy="126" r="5" /></g>
                <g class="dep-node lvl3"><circle class="ring" cx="262" cy="100" r="9" /><circle class="core" cx="262" cy="100" r="5" /></g>
                <!-- resource labels -->
                <text class="lbl" x="34" y="118" text-anchor="middle">Namespace</text>
                <text class="lbl" x="114" y="40" text-anchor="middle">ConfigMap</text>
                <text class="lbl" x="114" y="100" text-anchor="middle" dx="26">Secret</text>
                <text class="lbl" x="114" y="168" text-anchor="middle">ServiceAccount</text>
                <text class="lbl" x="200" y="64" text-anchor="middle">Deployment</text>
                <text class="lbl" x="200" y="146" text-anchor="middle">StatefulSet</text>
                <text class="lbl" x="262" y="118" text-anchor="middle">Ingress</text>
              </svg>
            </div>
          </div>
          <h3 class="ftitle font-medium text-lynq-text">Dependency-aware ordering</h3>
          <p class="fdesc text-lynq-dim">
            Declare <code>dependIds</code> and Lynq builds a DAG, applying
            resources in topological order and waiting for readiness gates.
          </p>
        </article>

        <!-- 4. One template, every node (central token + orbiting value pills) -->
        <article class="fcard span-2 flex flex-col rounded-lynq border border-lynq-border bg-lynq-card p-6">
          <div class="fviz">
            <div class="tform">
              <!-- The template document: one LynqForm defining several resources -->
              <div class="tform-doc">
                <div class="tform-bar">
                  <span class="tform-file">web-stack.yaml</span>
                  <span class="tform-kind">LynqForm</span>
                </div>
                <div
                  v-for="(l, i) in tplLines"
                  :key="l.key"
                  class="tline"
                  :class="'tl' + (i + 1)"
                >
                  <span class="tl-key">{{ l.key }}:</span>
                  <span class="tl-val">{{ l.val }}</span>
                </div>
              </div>
              <!-- renders → -->
              <div class="tform-flow" aria-hidden="true">
                <span class="tf-label">renders</span>
                <span class="tf-arrow">&#8595;</span>
              </div>
              <!-- Generated resources, stamped out one per definition -->
              <div class="tform-out">
                <span
                  v-for="(l, i) in tplLines"
                  :key="l.res"
                  class="tres"
                  :class="'o' + (i + 1)"
                >{{ l.res }}</span>
              </div>
            </div>
          </div>
          <h3 class="ftitle font-medium text-lynq-text">One template, every node</h3>
          <p class="fdesc text-lynq-dim">
            Write a LynqForm once. Your columns render into a full resource set
            per row — Deployments, Services, Ingresses, and more.
          </p>
        </article>

        <!-- 5. Every resource tracked (live per-resource status list) -->
        <article class="fcard span-2 flex flex-col rounded-lynq border border-lynq-border bg-lynq-card p-6">
          <div class="fviz">
            <!-- Lynq reports every resource's real state. Each badge resolves from
                 Pending to its actual status — Ready, Failed, Conflict, Skipped —
                 so problems surface per resource, live. -->
            <div class="trk" aria-hidden="true">
              <div class="trk-head">
                <span class="trk-watch"></span>
                <span class="trk-node">acme-corp-web-stack</span>
              </div>
              <div class="trk-row r1"><span class="tk-kind">Deployment</span><span class="tk-badge"></span></div>
              <div class="trk-row r2"><span class="tk-kind">Service</span><span class="tk-badge"></span></div>
              <div class="trk-row r3"><span class="tk-kind">Ingress</span><span class="tk-badge"></span></div>
              <div class="trk-row r4"><span class="tk-kind">PVC</span><span class="tk-badge"></span></div>
              <div class="trk-row r5"><span class="tk-kind">Job</span><span class="tk-badge"></span></div>
            </div>
          </div>
          <h3 class="ftitle font-medium text-lynq-text">Every resource tracked</h3>
          <p class="fdesc text-lynq-dim">
            Each LynqNode reports ready, pending, failed, skipped, and conflicts —
            so you always know the real state, per row.
          </p>
        </article>
      </div>
    </div>
  </section>
</template>

<script setup>
import SectionHeader from '../primitives/SectionHeader.vue'
// Held as a constant so the Go-template literal isn't parsed as a Vue mustache
// interpolation; rendered via v-text.
const uidLiteral = '{{ .uid }}'
// Template lines for the "One template, every node" card. Each defined resource
// renders into a created resource. Interpolating these strings is safe — Vue
// only parses {{ }} in template source, not in dynamic string values.
const tplLines = [
  { key: 'deployment', val: `${uidLiteral}-app`, res: 'Deployment' },
  { key: 'service', val: `${uidLiteral}-svc`, res: 'Service' },
  { key: 'ingress', val: `${uidLiteral}-web`, res: 'Ingress' },
]
// Pure-CSS looping illustrations (SSR-safe, no JS). All motion is gated behind
// `@media (prefers-reduced-motion: no-preference)`, so reduced-motion users see
// a calm static final frame. Illustration panels are fixed-height → zero CLS.
</script>

<style scoped>
.feature-grid-section {
  padding: var(--lynq-section-y) 2rem;
}
.fg-inner {
  /* literal (not var(--lynq-container)) — the custom prop does not resolve in
     scoped CSS on this page, which was letting the grid span full width. */
  max-width: 1040px;
  scroll-margin-top: 5rem;
}

/* windflow layout: 6-col grid, 2 wide cards on top, 3 on the bottom. Grid gap
   and margin-top are on the template (mt-14 gap-5). */
.fg-grid {
  grid-template-columns: repeat(6, 1fr);
}
.span-3 { grid-column: span 3; }
.span-2 { grid-column: span 2; }

/* Illustration panel — fixed height, clipped, its own inset "screen" frame.
   Matches windflow: 14px radius, a slightly-raised #0a0a0a inset over the
   #050505 card, 20px below (mb-5). */
.fviz {
  position: relative;
  height: 200px;
  margin-bottom: 20px;
  border: 1px solid var(--lynq-border);
  border-radius: 14px;
  background: #0a0a0a;
  overflow: hidden;
}

/* color is on the template (text-lynq-text). Sized to windflow's restrained
   card title: 16px / 500 / normal tracking. font-weight is set here (not via
   the font-medium utility) because VitePress's unlayered base `h3{600}` beats
   layered utilities — scoped specificity wins instead. */
.ftitle {
  margin: 0 0 0.5rem;
  font-size: 1rem;
  font-weight: 500;
  letter-spacing: normal;
}
/* windflow card body copy: 14px, muted #888, 1.625 leading. */
.fdesc {
  margin: 0;
  font-size: 0.875rem;
  line-height: 1.625;
  color: #888888;
}
.fdesc code {
  font-family: var(--lynq-mono);
  font-size: 0.86em;
  color: var(--lynq-accent);
  background: rgba(51, 172, 168, 0.12);
  padding: 0.05em 0.35em;
  border-radius: 5px;
}

/* ============================ 1. Database table (polled row by row) ============================ */
.dbwin {
  position: absolute;
  inset: 22px 26px;
  border: 1px solid var(--lynq-border);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.03);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.db-title {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 12px;
  border-bottom: 1px solid var(--lynq-border);
}
.db-ico {
  width: 15px; height: 15px;
  fill: none;
  stroke: var(--lynq-accent);
  stroke-width: 1.6;
}
.db-name {
  font-family: var(--lynq-mono);
  font-size: 0.78rem;
  color: var(--lynq-text);
}
.db-tag {
  margin-left: auto;
  font-family: var(--lynq-mono);
  font-size: 0.62rem;
  color: var(--lynq-text-faint);
  border: 1px solid var(--lynq-border);
  border-radius: 5px;
  padding: 1px 6px;
}
.db-tbl {
  position: relative;
  flex: 1;
  padding: 4px 0;
}
.db-r {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 26px;
  padding: 0 14px;
  font-family: var(--lynq-mono);
  font-size: 0.72rem;
  color: var(--lynq-text-dim);
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}
.db-r > span:first-child { flex: 1; }
.db-h {
  color: var(--lynq-text-faint);
  font-size: 0.66rem;
  text-transform: none;
}
.db-flag { width: 68px; text-align: right; }
.db-flag.on { color: var(--lynq-green); }
.db-flag.off { color: var(--lynq-text-faint); }
/* The "read head" — a highlight bar that steps through each data row, as if
   Lynq is polling the table row by row. */
.db-scan {
  position: absolute;
  left: 6px; right: 6px;
  height: 26px;
  border-radius: 7px;
  border-left: 2px solid var(--lynq-accent);
  background: linear-gradient(90deg, rgba(51, 172, 168, 0.16), rgba(51, 172, 168, 0.04));
  top: 30px; /* first data row (header is row 0, 26px + 4px pad) */
  opacity: 0;
}
@media (prefers-reduced-motion: no-preference) {
  /* literal easing: var() inside a scoped `animation` shorthand breaks Vue's
     @keyframes-name rewrite, silently disabling the animation. */
  .db-scan { animation: db-read 3.6s cubic-bezier(0.22, 1, 0.36, 1) infinite; }
}
@keyframes db-read {
  0%           { transform: translateY(0);   opacity: 0; }
  8%, 25%      { transform: translateY(0);   opacity: 1; }
  33%, 50%     { transform: translateY(26px); opacity: 1; }
  58%, 75%     { transform: translateY(52px); opacity: 1; }
  83%          { transform: translateY(52px); opacity: 0; }
  100%         { transform: translateY(0);   opacity: 0; }
}

/* Blinking read caret in the title bar (constant polling). */
.db-caret {
  width: 7px; height: 13px;
  margin-left: 1px;
  background: var(--lynq-accent);
  border-radius: 1px;
}
@media (prefers-reduced-motion: no-preference) {
  .db-caret { animation: caret-blink 1s steps(1) infinite; }
}
@keyframes caret-blink { 0%,50% { opacity: 1; } 51%,100% { opacity: 0; } }

/* Active rows emit a node dot flying to the right edge — "row → LynqNode". */
.db-r { position: relative; }
.db-emit {
  position: absolute;
  right: 12px; top: 50%;
  width: 6px; height: 6px;
  margin-top: -3px;
  border-radius: 50%;
  background: var(--lynq-green);
  box-shadow: 0 0 8px 1px rgba(16, 185, 129, 0.7);
  opacity: 0;
}
@media (prefers-reduced-motion: no-preference) {
  .em1 { animation: emit-node 3.6s cubic-bezier(0.22, 1, 0.36, 1) infinite 0.5s; }
  .em2 { animation: emit-node 3.6s cubic-bezier(0.22, 1, 0.36, 1) infinite 1.4s; }
  .db-flag.on { animation: flag-pulse 3.6s ease-in-out infinite; }
}
@keyframes emit-node {
  0%, 6%   { opacity: 0; transform: translateX(0) scale(0.6); }
  16%      { opacity: 1; transform: translateX(6px) scale(1); }
  55%      { opacity: 0; transform: translateX(40px) scale(0.7); }
  100%     { opacity: 0; transform: translateX(40px) scale(0.7); }
}
@keyframes flag-pulse {
  0%, 100% { text-shadow: none; }
  50%      { text-shadow: 0 0 8px rgba(16, 185, 129, 0.8); }
}

/* ============================ 2. Server-Side Apply (per-field ownership) ============================ */
.ssa {
  position: absolute;
  inset: 14px 22px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 8px;
}
.ssa-doc {
  position: relative;
  flex: 0 0 auto;
  border: 1px solid var(--lynq-border);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.03);
  overflow: hidden;
}
.ssa-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 5px 10px;
  border-bottom: 1px solid var(--lynq-border);
}
.ssa-file {
  font-family: var(--lynq-mono);
  font-size: 0.66rem;
  color: var(--lynq-text-dim);
}
.ssa-apply {
  font-family: var(--lynq-mono);
  font-size: 0.56rem;
  color: #33aca8;
  border: 1px solid rgba(51, 172, 168, 0.4);
  border-radius: 4px;
  padding: 1px 6px;
}
.ssa-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 10px;
  border-left: 2px solid transparent;
  font-family: var(--lynq-mono);
  font-size: 0.62rem;
}
.ssa-k { color: var(--lynq-text-dim); }
.ssa-v { color: var(--lynq-text); }
.ssa-v::before { content: '"'; opacity: 0.4; }
.ssa-v::after { content: '"'; opacity: 0.4; }
.ssa-own {
  margin-left: auto;
  font-size: 0.54rem;
  padding: 0 6px;
  border-radius: 4px;
  line-height: 1.5;
}
.o-lynq {
  color: #33aca8;
  border: 1px solid rgba(51, 172, 168, 0.4);
  background: rgba(51, 172, 168, 0.08);
}
.o-other {
  color: var(--lynq-amber);
  border: 1px solid rgba(245, 158, 11, 0.4);
  background: rgba(245, 158, 11, 0.08);
}
/* the field owned by another manager is visually set apart and never lit */
.sr-keep {
  border-left: 2px dashed rgba(255, 255, 255, 0.14);
}
.sr-keep .ssa-k,
.sr-keep .ssa-v { opacity: 0.55; }
/* teal apply sweep that runs down the manifest */
.ssa-scan {
  position: absolute;
  left: 0;
  right: 0;
  top: 30%;
  height: 2px;
  background: linear-gradient(90deg, transparent, #33aca8, transparent);
  box-shadow: 0 0 12px 1px rgba(51, 172, 168, 0.6);
  opacity: 0;
}
.ssa-fm {
  display: flex;
  align-items: center;
  gap: 6px;
  font-family: var(--lynq-mono);
  font-size: 0.58rem;
  color: var(--lynq-text-faint);
}
.ssa-fm b { color: #33aca8; font-weight: 600; }
.ssa-fm em { color: var(--lynq-amber); font-style: normal; }
.fm-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #33aca8;
  flex: 0 0 auto;
}

/* Settled/base state = the three lynq fields applied (teal). Reduced-motion
   users see this; the sweep below replays the field-by-field apply. */
.sr1, .sr2, .sr3 {
  border-left-color: #33aca8;
  background: rgba(51, 172, 168, 0.06);
}

@media (prefers-reduced-motion: no-preference) {
  .sr1 { animation: ssa-apply-1 4.2s ease-in-out infinite; }
  .sr2 { animation: ssa-apply-2 4.2s ease-in-out infinite; }
  .sr3 { animation: ssa-apply-3 4.2s ease-in-out infinite; }
  .ssa-scan { animation: ssa-sweep 4.2s cubic-bezier(0.4, 0, 0.2, 1) infinite; }
}
/* rows light as the sweep passes, hold, then reset for the next loop */
@keyframes ssa-apply-1 {
  0%, 16%   { border-left-color: transparent; background: transparent; }
  20%       { border-left-color: #33aca8; background: rgba(51, 172, 168, 0.16); }
  26%, 90%  { border-left-color: #33aca8; background: rgba(51, 172, 168, 0.06); }
  96%, 100% { border-left-color: transparent; background: transparent; }
}
@keyframes ssa-apply-2 {
  0%, 32%   { border-left-color: transparent; background: transparent; }
  36%       { border-left-color: #33aca8; background: rgba(51, 172, 168, 0.16); }
  42%, 90%  { border-left-color: #33aca8; background: rgba(51, 172, 168, 0.06); }
  96%, 100% { border-left-color: transparent; background: transparent; }
}
@keyframes ssa-apply-3 {
  0%, 48%   { border-left-color: transparent; background: transparent; }
  52%       { border-left-color: #33aca8; background: rgba(51, 172, 168, 0.16); }
  58%, 90%  { border-left-color: #33aca8; background: rgba(51, 172, 168, 0.06); }
  96%, 100% { border-left-color: transparent; background: transparent; }
}
@keyframes ssa-sweep {
  0%, 8%    { top: 30%; opacity: 0; }
  14%       { opacity: 1; }
  70%       { top: 92%; opacity: 1; }
  78%, 100% { top: 92%; opacity: 0; }
}

/* ============================ 3. Dependency DAG (windflow-style node graph) ============================ */
.dep {
  position: absolute;
  inset: 14px 16px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.dep-svg {
  width: 100%;
  height: 100%;
  overflow: visible;
}
/* Hollow ring nodes (faint teal). The fill "core" lights in topological order. */
.dep-node .ring {
  fill: none;
  stroke: rgba(51, 172, 168, 0.3);
  stroke-width: 1.4;
}
.dep-node .core {
  fill: #33aca8;
  transform-box: fill-box;
  transform-origin: center;
}
.lbl {
  fill: rgba(255, 255, 255, 0.55);
  font-family: var(--lynq-mono);
  font-size: 9px;
  letter-spacing: 0.02em;
}

/* Base state = the settled/completed DAG (all cores filled, all edges drawn).
   This is what reduced-motion users see; the animation below starts from empty
   and plays the level-by-level build-up for everyone else. */
.dep-node .core { opacity: 1; }
.ed { stroke-dashoffset: 0; }

@media (prefers-reduced-motion: no-preference) {
  /* Nodes fill by dependency level — every node in a level shares one keyframe,
     so the three level-1 resources (and the two level-2 workloads) light up in
     parallel, which is the whole point: same level = applied concurrently.
     literal easing/colors: var() inside a scoped `animation` shorthand breaks
     Vue's @keyframes-name rewrite and silently disables the animation. */
  .lvl0 .core { animation: dep-core-l0 6s cubic-bezier(0.22, 1, 0.36, 1) infinite; }
  .lvl1 .core { animation: dep-core-l1 6s cubic-bezier(0.22, 1, 0.36, 1) infinite; }
  .lvl2 .core { animation: dep-core-l2 6s cubic-bezier(0.22, 1, 0.36, 1) infinite; }
  .lvl3 .core { animation: dep-core-l3 6s cubic-bezier(0.22, 1, 0.36, 1) infinite; }
  .e-l1 { animation: dep-edge-l1 6s ease-in-out infinite; }
  .e-l2 { animation: dep-edge-l2 6s ease-in-out infinite; }
  .e-l3 { animation: dep-edge-l3 6s ease-in-out infinite; }
}

/* Node fill: hidden → glow flash on "apply" → steady, holds, then resets. */
@keyframes dep-core-l0 {
  0%, 4%    { opacity: 0; transform: scale(0.4); filter: none; }
  6%        { opacity: 1; transform: scale(1.45); filter: drop-shadow(0 0 6px rgba(79, 209, 203, 0.9)); }
  11%, 88%  { opacity: 1; transform: scale(1); filter: drop-shadow(0 0 2px rgba(51, 172, 168, 0.55)); }
  95%, 100% { opacity: 0; transform: scale(0.4); filter: none; }
}
@keyframes dep-core-l1 {
  0%, 22%   { opacity: 0; transform: scale(0.4); filter: none; }
  24%       { opacity: 1; transform: scale(1.45); filter: drop-shadow(0 0 6px rgba(79, 209, 203, 0.9)); }
  29%, 88%  { opacity: 1; transform: scale(1); filter: drop-shadow(0 0 2px rgba(51, 172, 168, 0.55)); }
  95%, 100% { opacity: 0; transform: scale(0.4); filter: none; }
}
@keyframes dep-core-l2 {
  0%, 40%   { opacity: 0; transform: scale(0.4); filter: none; }
  42%       { opacity: 1; transform: scale(1.45); filter: drop-shadow(0 0 6px rgba(79, 209, 203, 0.9)); }
  47%, 88%  { opacity: 1; transform: scale(1); filter: drop-shadow(0 0 2px rgba(51, 172, 168, 0.55)); }
  95%, 100% { opacity: 0; transform: scale(0.4); filter: none; }
}
@keyframes dep-core-l3 {
  0%, 56%   { opacity: 0; transform: scale(0.4); filter: none; }
  58%       { opacity: 1; transform: scale(1.45); filter: drop-shadow(0 0 6px rgba(79, 209, 203, 0.9)); }
  63%, 88%  { opacity: 1; transform: scale(1); filter: drop-shadow(0 0 2px rgba(51, 172, 168, 0.55)); }
  95%, 100% { opacity: 0; transform: scale(0.4); filter: none; }
}
/* Edge draw: pathLength=100, so dashoffset 100 = hidden, 0 = fully drawn. */
@keyframes dep-edge-l1 {
  0%, 12%   { stroke-dashoffset: 100; }
  20%, 88%  { stroke-dashoffset: 0; }
  94%, 100% { stroke-dashoffset: 100; }
}
@keyframes dep-edge-l2 {
  0%, 30%   { stroke-dashoffset: 100; }
  38%, 88%  { stroke-dashoffset: 0; }
  94%, 100% { stroke-dashoffset: 100; }
}
@keyframes dep-edge-l3 {
  0%, 48%   { stroke-dashoffset: 100; }
  56%, 88%  { stroke-dashoffset: 0; }
  94%, 100% { stroke-dashoffset: 100; }
}

/* ============================ 4. Template defines → resources created ============================ */
.tform {
  position: absolute;
  inset: 16px;
  display: flex;
  flex-direction: column;
  gap: 7px;
}
/* The template document */
.tform-doc {
  border: 1px solid var(--lynq-border);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.03);
  overflow: hidden;
}
.tform-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 5px 9px;
  border-bottom: 1px solid var(--lynq-border);
}
.tform-file {
  font-family: var(--lynq-mono);
  font-size: 0.66rem;
  color: var(--lynq-text-dim);
}
.tform-kind {
  font-family: var(--lynq-mono);
  font-size: 0.56rem;
  color: var(--lynq-accent);
  border: 1px solid rgba(51, 172, 168, 0.4);
  border-radius: 4px;
  padding: 0 5px;
}
.tline {
  display: flex;
  gap: 6px;
  align-items: baseline;
  padding: 2px 9px;
  font-family: var(--lynq-mono);
  font-size: 0.62rem;
  line-height: 1.5;
  border-left: 2px solid transparent;
  transition: background 0.3s var(--lynq-ease), border-color 0.3s var(--lynq-ease);
}
.tform-doc { padding-bottom: 2px; }
.tl-key { color: var(--lynq-text-dim); }
.tl-val { color: var(--lynq-text); }
.tl-val::before { content: '"'; opacity: 0.4; }
.tl-val::after { content: '"'; opacity: 0.4; }

/* renders ↓ */
.tform-flow {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: var(--lynq-text-faint);
  font-family: var(--lynq-mono);
  font-size: 0.6rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.tf-arrow { color: var(--lynq-accent); }

/* Generated resources — one chip per template definition */
.tform-out {
  display: flex;
  gap: 6px;
  justify-content: center;
  flex-wrap: wrap;
}
.tres {
  font-family: var(--lynq-mono);
  font-size: 0.62rem;
  color: var(--lynq-green);
  padding: 0.18rem 0.5rem;
  border-radius: 6px;
  border: 1px solid rgba(16, 185, 129, 0.4);
  background: rgba(16, 185, 129, 0.08);
  opacity: 0;
  transform: translateY(6px) scale(0.9);
}

/* Each definition lights up, then its resource pops into existence. Loop 4.5s. */
@media (prefers-reduced-motion: no-preference) {
  .tl1 { animation: def-lit 4.5s linear infinite; }
  .tl2 { animation: def-lit 4.5s linear infinite; animation-delay: 1s; }
  .tl3 { animation: def-lit 4.5s linear infinite; animation-delay: 2s; }
  .o1 { animation: res-pop 4.5s cubic-bezier(0.22, 1, 0.36, 1) infinite; }
  .o2 { animation: res-pop 4.5s cubic-bezier(0.22, 1, 0.36, 1) infinite; animation-delay: 1s; }
  .o3 { animation: res-pop 4.5s cubic-bezier(0.22, 1, 0.36, 1) infinite; animation-delay: 2s; }
}
@keyframes def-lit {
  0%, 4%    { background: transparent; border-left-color: transparent; }
  9%        { background: rgba(51, 172, 168, 0.14); border-left-color: var(--lynq-accent); }
  60%       { background: rgba(51, 172, 168, 0.14); border-left-color: var(--lynq-accent); }
  70%, 100% { background: transparent; border-left-color: transparent; }
}
@keyframes res-pop {
  0%, 6%    { opacity: 0; transform: translateY(6px) scale(0.9); }
  14%       { opacity: 1; transform: translateY(0) scale(1); }
  60%       { opacity: 1; transform: translateY(0) scale(1); }
  72%, 100% { opacity: 0; transform: translateY(-4px) scale(0.95); }
}

/* ============================ 5. Live per-resource status list ============================ */
.trk {
  position: absolute;
  inset: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.trk-head {
  display: flex;
  align-items: center;
  gap: 7px;
  padding-bottom: 7px;
  margin-bottom: 3px;
  border-bottom: 1px solid var(--lynq-border);
}
/* "watching" pulse — Lynq is observing the resources */
.trk-watch {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #33aca8;
  flex: 0 0 auto;
}
.trk-node {
  font-family: var(--lynq-mono);
  font-size: 0.64rem;
  color: var(--lynq-text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.trk-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  height: 24px;
}
.tk-kind {
  font-family: var(--lynq-mono);
  font-size: 0.68rem;
  color: var(--lynq-text-dim);
}
.tk-badge { flex: 0 0 auto; }
/* the status pill lives on ::after so a single keyframe can swap text + colours */
.tk-badge::after {
  content: 'READY';
  font-family: var(--lynq-mono);
  font-size: 0.52rem;
  letter-spacing: 0.05em;
  padding: 1px 6px;
  border-radius: 5px;
  border: 1px solid transparent;
}
/* settled state (also the reduced-motion frame): each resource's real status */
.r1 .tk-badge::after { content: 'READY';    color: #33aca8; background: rgba(51,172,168,0.1);  border-color: rgba(51,172,168,0.4); }
.r2 .tk-badge::after { content: 'READY';    color: #33aca8; background: rgba(51,172,168,0.1);  border-color: rgba(51,172,168,0.4); }
.r3 .tk-badge::after { content: 'FAILED';   color: #ef4444; background: rgba(239,68,68,0.1);   border-color: rgba(239,68,68,0.4); }
.r4 .tk-badge::after { content: 'CONFLICT'; color: #f59e0b; background: rgba(245,158,11,0.1);  border-color: rgba(245,158,11,0.4); }
.r5 .tk-badge::after { content: 'SKIPPED';  color: #64748b; background: rgba(100,116,139,0.14); border-color: rgba(100,116,139,0.4); }

@media (prefers-reduced-motion: no-preference) {
  .trk-watch { animation: tk-watch 1.7s ease-out infinite; }
  .r1 .tk-badge::after { animation: tk-b1 6s ease-in-out infinite; }
  .r2 .tk-badge::after { animation: tk-b2 6s ease-in-out infinite; }
  .r3 .tk-badge::after { animation: tk-b3 6s ease-in-out infinite; }
  .r4 .tk-badge::after { animation: tk-b4 6s ease-in-out infinite; }
  .r5 .tk-badge::after { animation: tk-b5 6s ease-in-out infinite; }
}
@keyframes tk-watch {
  0%       { box-shadow: 0 0 0 0 rgba(51,172,168,0.6); }
  70%,100% { box-shadow: 0 0 0 5px rgba(51,172,168,0); }
}
/* each badge starts PENDING (amber) then resolves to its real status, staggered,
   holds, and resets — so Lynq is seen observing each resource in turn. */
@keyframes tk-b1 {
  0%,12%    { content: 'PENDING'; color: #f59e0b; background: rgba(245,158,11,0.1); border-color: rgba(245,158,11,0.4); }
  16%,88%   { content: 'READY'; color: #33aca8; background: rgba(51,172,168,0.1); border-color: rgba(51,172,168,0.4); }
  94%,100%  { content: 'PENDING'; color: #f59e0b; background: rgba(245,158,11,0.1); border-color: rgba(245,158,11,0.4); }
}
@keyframes tk-b2 {
  0%,24%    { content: 'PENDING'; color: #f59e0b; background: rgba(245,158,11,0.1); border-color: rgba(245,158,11,0.4); }
  28%,88%   { content: 'READY'; color: #33aca8; background: rgba(51,172,168,0.1); border-color: rgba(51,172,168,0.4); }
  94%,100%  { content: 'PENDING'; color: #f59e0b; background: rgba(245,158,11,0.1); border-color: rgba(245,158,11,0.4); }
}
@keyframes tk-b3 {
  0%,36%    { content: 'PENDING'; color: #f59e0b; background: rgba(245,158,11,0.1); border-color: rgba(245,158,11,0.4); }
  40%,88%   { content: 'FAILED'; color: #ef4444; background: rgba(239,68,68,0.1); border-color: rgba(239,68,68,0.4); }
  94%,100%  { content: 'PENDING'; color: #f59e0b; background: rgba(245,158,11,0.1); border-color: rgba(245,158,11,0.4); }
}
@keyframes tk-b4 {
  0%,48%    { content: 'PENDING'; color: #f59e0b; background: rgba(245,158,11,0.1); border-color: rgba(245,158,11,0.4); }
  52%,88%   { content: 'CONFLICT'; color: #f59e0b; background: rgba(245,158,11,0.14); border-color: rgba(245,158,11,0.55); }
  94%,100%  { content: 'PENDING'; color: #f59e0b; background: rgba(245,158,11,0.1); border-color: rgba(245,158,11,0.4); }
}
@keyframes tk-b5 {
  0%,60%    { content: 'PENDING'; color: #f59e0b; background: rgba(245,158,11,0.1); border-color: rgba(245,158,11,0.4); }
  64%,88%   { content: 'SKIPPED'; color: #64748b; background: rgba(100,116,139,0.14); border-color: rgba(100,116,139,0.4); }
  94%,100%  { content: 'PENDING'; color: #f59e0b; background: rgba(245,158,11,0.1); border-color: rgba(245,158,11,0.4); }
}

/* ============================ responsive ============================ */
@media (max-width: 860px) {
  .fg-grid { grid-template-columns: 1fr; gap: 1rem; }
  .span-3, .span-2 { grid-column: auto; }
  .fviz { height: 180px; }
}
</style>
