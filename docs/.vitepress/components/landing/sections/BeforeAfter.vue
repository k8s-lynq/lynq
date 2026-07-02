<template>
  <section class="before-after scroll-mt-20 px-8">
    <div class="mx-auto" style="max-width: 1040px">
      <SectionHeader
        label="The Problem"
        title="Your Database Knows. Your Cluster Doesn't."
        subtitle="The moment a row changes, the cluster is out of date — until someone runs kubectl. Lynq closes that gap continuously: the cluster is always a reflection of the database."
        accent="amber"
      />

      <!-- Compact drift loop: the Database holds the desired state; the Cluster
           drifts out of sync (red), then Lynq reconciles it back to match (teal).
           Pure CSS; reduced-motion rests on the in-sync frame. -->
      <div class="drift" aria-hidden="true">
        <div class="drift-grid">
          <!-- Database — the source of truth, always correct -->
          <div class="col col-db">
            <div class="col-h"><span class="ch-dot ok"></span>Database<em>desired</em></div>
            <div class="row"><span class="r-id">acme-corp</span><span class="r-val">pro</span></div>
            <div class="row"><span class="r-id">beta-inc</span><span class="r-val">free</span></div>
            <div class="row"><span class="r-id">gamma-llc</span><span class="r-val">pro</span></div>
          </div>

          <!-- reconcile beam sweeps DB → cluster when Lynq closes the gap -->
          <div class="col-mid">
            <span class="mid-lbl">Lynq</span>
            <span class="mid-sub">reconciles</span>
          </div>
          <span class="beam"></span>

          <!-- Cluster — drifts without Lynq, then snaps back into sync -->
          <div class="col col-cl">
            <div class="col-h"><span class="ch-dot cl-dot"></span>Cluster<em>live</em></div>
            <div class="row crow c1"><span class="r-id">acme-corp</span><span class="r-st"></span></div>
            <div class="row crow c2"><span class="r-id">beta-inc</span><span class="r-st"></span></div>
            <div class="row crow c3"><span class="r-id">gamma-llc</span><span class="r-st"></span></div>
          </div>
        </div>

        <div class="drift-cap"></div>
      </div>
    </div>
  </section>
</template>

<script setup>
import SectionHeader from '../primitives/SectionHeader.vue'
</script>

<style scoped>
.before-after {
  padding-block: var(--lynq-section-y);
  position: relative;
}
/* "The Problem" section signature — a warm red/amber wash so drift reads as
   tension. Painted on ::before so the global section-transparency rule (which
   only clears `background`) leaves it intact. */
.before-after::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background:
    radial-gradient(52rem 32rem at 50% -4%, rgba(239, 68, 68, 0.1), transparent 62%),
    radial-gradient(40rem 30rem at 88% 104%, rgba(245, 158, 11, 0.07), transparent 60%);
}
.before-after > * {
  position: relative;
  z-index: 1;
}

/* ---- compact drift panel ---- */
.drift {
  max-width: 720px;
  margin: 0 auto;
  border: 1px solid var(--lynq-border);
  border-radius: var(--lynq-radius);
  background: rgba(255, 255, 255, 0.015);
  padding: 1.6rem 1.75rem 1.4rem;
}
.drift-grid {
  position: relative;
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  gap: 1.25rem;
  align-items: start;
}

.col {
  border: 1px solid var(--lynq-border);
  border-radius: 12px;
  background: #0c0c10;
  padding: 0.75rem 0.85rem;
  transition: border-color 0.4s ease;
}
.col-h {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  padding-bottom: 0.55rem;
  margin-bottom: 0.5rem;
  border-bottom: 1px solid var(--lynq-border);
  font-family: var(--lynq-mono);
  font-size: 0.72rem;
  color: var(--lynq-text);
}
.col-h em {
  margin-left: auto;
  font-style: normal;
  font-size: 0.56rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--lynq-text-faint);
}
.ch-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--lynq-text-faint);
}
.ch-dot.ok { background: #33aca8; }

.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  height: 26px;
  font-family: var(--lynq-mono);
  font-size: 0.72rem;
}
.r-id { color: var(--lynq-text-dim); }
.r-val { color: #33aca8; }

/* cluster status badge (::after so one keyframe swaps text + colour) */
.r-st::after {
  content: '✓ in sync';
  font-size: 0.56rem;
  letter-spacing: 0.03em;
  padding: 1px 6px;
  border-radius: 5px;
  color: #33aca8;
  border: 1px solid rgba(51, 172, 168, 0.4);
  background: rgba(51, 172, 168, 0.1);
}

/* middle reconcile label */
.col-mid {
  align-self: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.1rem;
  padding-top: 1.4rem;
}
.mid-lbl {
  font-family: var(--lynq-mono);
  font-size: 0.66rem;
  color: #33aca8;
}
.mid-sub {
  font-family: var(--lynq-mono);
  font-size: 0.52rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--lynq-text-faint);
}

/* the reconcile beam: a teal sweep DB → cluster */
.beam {
  position: absolute;
  top: 2rem;
  bottom: 0.5rem;
  left: 34%;
  width: 2px;
  border-radius: 2px;
  background: linear-gradient(180deg, transparent, #4fd1cb, transparent);
  box-shadow: 0 0 12px 2px rgba(79, 209, 203, 0.6);
  opacity: 0;
}

.drift-cap {
  margin-top: 1.1rem;
  text-align: center;
  font-family: var(--lynq-mono);
  font-size: 0.78rem;
  min-height: 1.2em;
}
.drift-cap::after {
  content: 'Cluster matches the database';
  color: #33aca8;
}

/* ==================== animation ====================
   Rest state (also reduced-motion) is fully in-sync. With motion, the cluster
   drifts red one row at a time, Lynq's beam sweeps across, and the rows snap
   back to teal — a tight ~7.5s drift → reconcile loop. */
@media (prefers-reduced-motion: no-preference) {
  .c1 .r-st::after { animation: st-drift-1 7.5s ease-in-out infinite; }
  .c2 .r-st::after { animation: st-drift-2 7.5s ease-in-out infinite; }
  .c3 .r-st::after { animation: st-drift-3 7.5s ease-in-out infinite; }
  .col-cl { animation: cl-border 7.5s ease-in-out infinite; }
  .beam { animation: beam-sweep 7.5s ease-in-out infinite; }
  .cl-dot { animation: cl-dot 7.5s ease-in-out infinite; }
  .drift-cap::after { animation: cap-swap 7.5s ease-in-out infinite; }
}

/* drift = red "out of sync", staggered on; reconcile back to teal together */
@keyframes st-drift-1 {
  0%, 12%   { content: '✓ in sync'; color: #33aca8; border-color: rgba(51,172,168,0.4); background: rgba(51,172,168,0.1); }
  17%, 52%  { content: '✕ drift'; color: #ef4444; border-color: rgba(239,68,68,0.45); background: rgba(239,68,68,0.12); }
  58%, 100% { content: '✓ in sync'; color: #33aca8; border-color: rgba(51,172,168,0.4); background: rgba(51,172,168,0.1); }
}
@keyframes st-drift-2 {
  0%, 17%   { content: '✓ in sync'; color: #33aca8; border-color: rgba(51,172,168,0.4); background: rgba(51,172,168,0.1); }
  22%, 52%  { content: '✕ drift'; color: #ef4444; border-color: rgba(239,68,68,0.45); background: rgba(239,68,68,0.12); }
  61%, 100% { content: '✓ in sync'; color: #33aca8; border-color: rgba(51,172,168,0.4); background: rgba(51,172,168,0.1); }
}
@keyframes st-drift-3 {
  0%, 22%   { content: '✓ in sync'; color: #33aca8; border-color: rgba(51,172,168,0.4); background: rgba(51,172,168,0.1); }
  27%, 52%  { content: '✕ drift'; color: #ef4444; border-color: rgba(239,68,68,0.45); background: rgba(239,68,68,0.12); }
  64%, 100% { content: '✓ in sync'; color: #33aca8; border-color: rgba(51,172,168,0.4); background: rgba(51,172,168,0.1); }
}
/* cluster panel border flushes red while drifted */
@keyframes cl-border {
  0%, 14%   { border-color: var(--lynq-border); }
  24%, 50%  { border-color: rgba(239, 68, 68, 0.4); }
  60%, 100% { border-color: rgba(51, 172, 168, 0.35); }
}
@keyframes cl-dot {
  0%, 14%   { background: #33aca8; }
  24%, 50%  { background: #ef4444; }
  60%, 100% { background: #33aca8; }
}
/* Lynq beam sweeps DB → cluster to close the gap */
@keyframes beam-sweep {
  0%, 50%   { left: 34%; opacity: 0; }
  54%       { left: 34%; opacity: 1; }
  62%       { left: 66%; opacity: 1; }
  66%, 100% { left: 66%; opacity: 0; }
}
/* one-line narration: in sync → drifted → reconciled */
@keyframes cap-swap {
  0%, 12%   { content: 'Cluster matches the database'; color: #33aca8; }
  20%, 50%  { content: 'A row changed — the cluster is now out of sync'; color: #ef4444; }
  60%, 100% { content: 'Lynq reconciled — cluster matches, zero drift'; color: #33aca8; }
}

@media (max-width: 620px) {
  .drift-grid { grid-template-columns: 1fr; gap: 0.9rem; }
  .col-mid { flex-direction: row; gap: 0.5rem; padding-top: 0; }
  .beam { display: none; }
}
</style>
