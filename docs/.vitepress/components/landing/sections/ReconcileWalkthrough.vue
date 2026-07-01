<template>
  <section class="reconcile bg-lynq-bg" id="reconcile">
    <div class="reconcile-inner mx-auto">
      <!-- Section header -->
      <header class="rw-head mx-auto mb-12 text-center">
        <span class="rw-kicker mb-4 inline-flex items-center gap-1.5 font-mono uppercase text-lynq-accent">
          <LineIcon name="sync" />
          Lifecycle
        </span>
        <h2 class="rw-title font-medium text-lynq-text">Watch a Row Become a Running App</h2>
        <p class="rw-subtitle m-0 text-lynq-dim">
          One database row, reconciled end to end — Lynq reads it, creates a
          LynqNode, applies the resources, and the app goes live.
        </p>
      </header>

      <!-- Seamless auto-playing pipeline. A single 11s loop drives everything: a
           playhead travels the top rail through four milestones while the four
           columns below build up in sync — row → LynqNode → resources → live app.
           Pure CSS (SSR-safe); reduced-motion shows the finished assembly. -->
      <div class="flow" aria-hidden="true">
        <!-- Timeline rail with four milestones -->
        <div class="flow-rail">
          <div class="rail-line">
            <span class="rail-fill"></span>
            <span class="rail-token"></span>
          </div>
          <div class="milestones">
            <div class="ms ms1"><span class="ms-dot"></span><span class="ms-lbl">Row</span></div>
            <div class="ms ms2"><span class="ms-dot"></span><span class="ms-lbl">Reconcile</span></div>
            <div class="ms ms3"><span class="ms-dot"></span><span class="ms-lbl">Apply</span></div>
            <div class="ms ms4"><span class="ms-dot"></span><span class="ms-lbl">Live</span></div>
          </div>
        </div>

        <!-- Four aligned stage columns -->
        <div class="flow-cols">
          <!-- 1 · a new row lands and LynqHub detects it -->
          <div class="col col1">
            <div class="card mini-db">
              <div class="mdb-bar">
                <span class="mdb-watch" title="LynqHub polling"></span>
                <span class="mdb-name">node_configs</span>
                <span class="mdb-tag">MySQL</span>
              </div>
              <div class="mdb-row">
                <span class="mdb-id">acme-corp</span>
                <span class="mdb-on">1</span>
                <span class="mdb-new">NEW</span>
              </div>
              <div class="mdb-meta">plan: pro · active</div>
              <!-- detection scan sweeps the table as LynqHub reads the new row -->
              <span class="mdb-scan"></span>
            </div>
          </div>

          <!-- 2 · the LynqNode it creates -->
          <div class="col col2">
            <div class="card cr-card">
              <span class="cr-kind">LynqNode</span>
              <span class="cr-name">acme-corp-web-stack</span>
              <span class="cr-by">created by LynqHub sync</span>
            </div>
          </div>

          <!-- 3 · the resources, Pending → Ready -->
          <div class="col col3">
            <div class="res res-app">
              <span class="res-k">Deployment</span>
              <span class="res-n">acme-corp-app</span>
              <span class="res-dot"></span>
            </div>
            <div class="res res-svc">
              <span class="res-k">Service</span>
              <span class="res-n">acme-corp-svc</span>
              <span class="res-dot"></span>
            </div>
            <div class="res res-web">
              <span class="res-k">Ingress</span>
              <span class="res-n">acme-corp-web</span>
              <span class="res-dot"></span>
            </div>
          </div>

          <!-- 4 · the running app -->
          <div class="col col4">
            <div class="card browser">
              <div class="br-bar">
                <span class="br-dots"><i></i><i></i><i></i></span>
                <span class="br-url">acme.example.com</span>
              </div>
              <div class="br-body">
                <span class="br-line l1"></span>
                <span class="br-line l2"></span>
                <span class="br-badge">● Running · 200</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Narration caption (crossfades per phase) -->
        <div class="flow-caption">
          <span class="cap cap1">LynqHub polls MySQL and detects new row <b>acme-corp</b></span>
          <span class="cap cap2">LynqHub creates <b>LynqNode/acme-corp-web-stack</b></span>
          <span class="cap cap3">Applying 3 resources in dependency order…</span>
          <span class="cap cap4"><b>acme.example.com</b> is live — 3 / 3 ready</span>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import LineIcon from '../primitives/LineIcon.vue'
</script>

<style scoped>
.reconcile {
  width: 100%;
  padding: 6rem 2rem;
  scroll-margin-top: 5rem;
}
.reconcile-inner {
  max-width: 1040px;
}

/* ---- Header (sizes literal; --lynq-h2 doesn't resolve in scoped CSS) ---- */
.rw-head { max-width: 40rem; }
.rw-kicker { gap: 0.4rem; font-size: 0.78rem; letter-spacing: 0.08em; }
.rw-title {
  font-size: clamp(2rem, 5vw, 3.75rem);
  font-weight: 500;
  line-height: 1.15;
  letter-spacing: -0.02em;
  margin: 0 0 0.9rem;
}
.rw-subtitle { font-size: 1.02rem; line-height: 1.6; }

/* ==================== Seamless pipeline stage ==================== */
.flow {
  position: relative;
  border: 1px solid var(--lynq-border);
  border-radius: var(--lynq-radius);
  background: rgba(255, 255, 255, 0.015);
  padding: 2rem 1.75rem 1.5rem;
}

/* ---- Timeline rail ---- */
.flow-rail { position: relative; margin-bottom: 1.75rem; }
.rail-line {
  position: absolute;
  top: 7px;
  left: 12.5%;
  right: 12.5%;
  height: 2px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 2px;
}
.rail-fill {
  position: absolute;
  left: 0;
  top: 0;
  height: 100%;
  width: 100%;
  transform-origin: left;
  background: linear-gradient(90deg, #33aca8, #4fd1cb);
  border-radius: 2px;
}
.rail-token {
  position: absolute;
  top: 50%;
  left: 100%;
  width: 10px;
  height: 10px;
  margin: -5px 0 0 -5px;
  border-radius: 50%;
  background: #4fd1cb;
  box-shadow: 0 0 10px 2px rgba(79, 209, 203, 0.7);
  opacity: 0;
}
.milestones { position: relative; display: flex; }
.ms {
  flex: 1 1 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
}
.ms-dot {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  border: 2px solid #33aca8;
  background: #33aca8;
  box-sizing: border-box;
}
.ms-lbl {
  font-family: var(--lynq-mono);
  font-size: 0.68rem;
  letter-spacing: 0.04em;
  color: var(--lynq-text-faint);
}

/* ---- Stage columns ---- */
.flow-cols { display: flex; align-items: stretch; min-height: 176px; }
.col {
  flex: 1 1 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 0.5rem;
  padding: 0 0.6rem;
}
.card {
  border: 1px solid var(--lynq-border);
  border-radius: 10px;
  background: #0c0c10;
}

/* 1 · mini database + LynqHub detection */
.mini-db { position: relative; overflow: hidden; }
.mdb-bar {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 5px 9px;
  border-bottom: 1px solid var(--lynq-border);
}
/* pulsing "poll" dot — LynqHub is watching the table */
.mdb-watch {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #33aca8;
  flex: 0 0 auto;
}
.mdb-name { flex: 1; font-family: var(--lynq-mono); font-size: 0.66rem; color: var(--lynq-text-dim); }
.mdb-tag {
  flex: 0 0 auto;
  font-family: var(--lynq-mono);
  font-size: 0.54rem;
  color: var(--lynq-text-faint);
  border: 1px solid var(--lynq-border);
  border-radius: 4px;
  padding: 0 5px;
}
.mdb-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 6px 9px;
  font-family: var(--lynq-mono);
  font-size: 0.72rem;
  color: var(--lynq-text);
  border-left: 2px solid #33aca8;
}
.mdb-id { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mdb-on { flex: 0 0 auto; color: #33aca8; }
.mdb-new {
  flex: 0 0 auto;
  display: inline-block;
  font-size: 0.5rem;
  letter-spacing: 0.06em;
  color: #4fd1cb;
  border: 1px solid rgba(79, 209, 203, 0.5);
  background: rgba(79, 209, 203, 0.12);
  border-radius: 3px;
  padding: 0 4px;
  opacity: 0; /* only shown by the insert animation */
}
.mdb-meta {
  padding: 4px 9px 7px;
  font-family: var(--lynq-mono);
  font-size: 0.6rem;
  color: var(--lynq-text-faint);
}
/* detection scan line that sweeps the table as LynqHub reads the new row */
.mdb-scan {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 2px;
  background: linear-gradient(180deg, transparent, #4fd1cb, transparent);
  box-shadow: 0 0 10px 2px rgba(79, 209, 203, 0.6);
  opacity: 0;
}

/* 2 · LynqNode CR card */
.cr-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.3rem;
  padding: 0.7rem 0.75rem;
}
.cr-kind {
  font-family: var(--lynq-mono);
  font-size: 0.56rem;
  color: #4fd1cb;
  border: 1px solid rgba(79, 209, 203, 0.4);
  border-radius: 4px;
  padding: 1px 6px;
}
.cr-name {
  font-family: var(--lynq-mono);
  font-size: 0.74rem;
  color: var(--lynq-text);
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.cr-by { font-size: 0.6rem; color: var(--lynq-text-faint); }

/* 3 · resources — status conveyed by the dot (amber pending → teal ready) */
.res {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 6px 9px;
  border: 1px solid var(--lynq-border);
  border-radius: 8px;
  background: #0c0c10;
}
.res-k {
  font-family: var(--lynq-mono);
  font-size: 0.58rem;
  color: var(--lynq-text-faint);
  flex: 0 0 auto;
}
.res-n {
  font-family: var(--lynq-mono);
  font-size: 0.62rem;
  color: var(--lynq-text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.res-dot {
  margin-left: auto;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #33aca8;
  flex: 0 0 auto;
}

/* 4 · running-app browser */
.browser { overflow: hidden; }
.br-bar {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 5px 8px;
  background: #16161c;
  border-bottom: 1px solid var(--lynq-border);
}
.br-dots { display: flex; gap: 3px; }
.br-dots i { width: 6px; height: 6px; border-radius: 50%; background: rgba(255, 255, 255, 0.2); }
.br-url {
  font-family: var(--lynq-mono);
  font-size: 0.56rem;
  color: var(--lynq-text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.br-body {
  display: flex;
  flex-direction: column;
  gap: 7px;
  padding: 0.7rem;
}
.br-line { height: 6px; border-radius: 3px; background: rgba(255, 255, 255, 0.08); }
.br-line.l1 { width: 65%; }
.br-line.l2 { width: 85%; }
.br-badge {
  align-self: flex-start;
  margin-top: 2px;
  font-family: var(--lynq-mono);
  font-size: 0.58rem;
  color: #33aca8;
  border: 1px solid rgba(51, 172, 168, 0.4);
  background: rgba(51, 172, 168, 0.08);
  border-radius: 5px;
  padding: 1px 7px;
}

/* ---- Caption ---- */
.flow-caption {
  position: relative;
  height: 1.4rem;
  margin-top: 1.5rem;
  text-align: center;
}
.cap {
  position: absolute;
  inset: 0;
  font-size: 0.86rem;
  color: var(--lynq-text-dim);
  opacity: 0;
}
.cap4 { opacity: 1; } /* settled default (reduced-motion) */
.cap b { color: var(--lynq-text); font-weight: 500; }
.cap code {
  font-family: var(--lynq-mono);
  font-size: 0.82em;
  color: var(--lynq-accent);
  background: rgba(79, 209, 203, 0.1);
  border-radius: 4px;
  padding: 0.05em 0.35em;
}

/* ==================== Animation (single 11s loop; absolute-timed) ====================
   Base styles above are the finished assembly, which is what reduced-motion
   users see. When motion is allowed, every element plays the same 11s cycle
   with its phase baked into the keyframe %s, so the whole scene stays in lockstep
   and resets together — no drift. literal easing in the shorthand (a var() there
   breaks Vue's scoped @keyframes-name rewrite). */
@media (prefers-reduced-motion: no-preference) {
  .rail-fill { animation: fw-fill 11s cubic-bezier(0.65, 0, 0.35, 1) infinite; }
  .rail-token { animation: fw-token 11s cubic-bezier(0.65, 0, 0.35, 1) infinite; }
  /* detection beat: row inserts, LynqHub scans + flags it NEW, dot keeps polling */
  .mdb-row { animation: fw-row-insert 11s ease-out infinite; }
  .mdb-new { animation: fw-new 11s ease-out infinite; }
  .mdb-scan { animation: fw-scan 11s ease-in-out infinite; }
  .mdb-watch { animation: fw-watch 2.2s ease-out infinite; }
  .ms1 .ms-dot { animation: fw-dot1 11s ease-out infinite; }
  .ms2 .ms-dot { animation: fw-dot2 11s ease-out infinite; }
  .ms3 .ms-dot { animation: fw-dot3 11s ease-out infinite; }
  .ms4 .ms-dot { animation: fw-dot4 11s ease-out infinite; }
  .col1 { animation: fw-col1 11s ease-out infinite; }
  .col2 { animation: fw-col2 11s ease-out infinite; }
  .col3 { animation: fw-col3 11s ease-out infinite; }
  .col4 { animation: fw-col4 11s ease-out infinite; }
  .res-app .res-dot { animation: fw-rd-app 11s ease-out infinite; }
  .res-svc .res-dot { animation: fw-rd-svc 11s ease-out infinite; }
  .res-web .res-dot { animation: fw-rd-web 11s ease-out infinite; }
  .br-badge { animation: fw-badge 11s ease-out infinite; }
  .cap1 { animation: fw-cap1 11s linear infinite; }
  .cap2 { animation: fw-cap2 11s linear infinite; }
  .cap3 { animation: fw-cap3 11s linear infinite; }
  .cap4 { animation: fw-cap4 11s linear infinite; }
}

/* playhead fill grows to each milestone, dwells during work, resets */
@keyframes fw-fill {
  0%, 2%    { transform: scaleX(0); }
  16%       { transform: scaleX(0); }
  24%, 34%  { transform: scaleX(0.3333); }
  42%, 64%  { transform: scaleX(0.6667); }
  72%, 92%  { transform: scaleX(1); }
  96%, 100% { transform: scaleX(0); }
}
@keyframes fw-token {
  0%, 2%    { left: 0%; opacity: 0; }
  4%, 16%   { left: 0%; opacity: 1; }
  24%, 34%  { left: 33.33%; opacity: 1; }
  42%, 64%  { left: 66.67%; opacity: 1; }
  72%, 92%  { left: 100%; opacity: 1; }
  95%, 100% { left: 100%; opacity: 0; }
}
/* the new row slides in (INSERT) with a teal flash */
@keyframes fw-row-insert {
  0%, 6%    { opacity: 0; transform: translateY(-8px); background: transparent; }
  10%       { opacity: 1; transform: translateY(0); background: rgba(51, 172, 168, 0.22); }
  14%, 90%  { opacity: 1; transform: translateY(0); background: transparent; }
  95%, 100% { opacity: 0; transform: translateY(-8px); background: transparent; }
}
/* NEW flag pops on insert, fades once it's been reconciled */
@keyframes fw-new {
  0%, 9%    { opacity: 0; transform: scale(0.7); }
  12%       { opacity: 1; transform: scale(1.15); }
  15%, 44%  { opacity: 1; transform: scale(1); }
  50%, 100% { opacity: 0; transform: scale(0.7); }
}
/* detection scan sweeps left→right across the table */
@keyframes fw-scan {
  0%, 11%   { left: 0; opacity: 0; }
  13%       { left: 0; opacity: 1; }
  19%       { left: 100%; opacity: 1; }
  21%, 100% { left: 100%; opacity: 0; }
}
/* LynqHub poll indicator — steady ping */
@keyframes fw-watch {
  0%        { box-shadow: 0 0 0 0 rgba(51, 172, 168, 0.6); }
  70%, 100% { box-shadow: 0 0 0 5px rgba(51, 172, 168, 0); }
}
/* milestone dots: dim → glow flash on arrival → steady teal → dim on reset */
@keyframes fw-dot1 {
  0%, 1%    { border-color: rgba(255,255,255,0.2); background: var(--lynq-bg); box-shadow: none; transform: scale(1); }
  2%        { border-color: #4fd1cb; background: #4fd1cb; box-shadow: 0 0 8px 2px rgba(79,209,203,0.7); transform: scale(1.25); }
  6%, 90%   { border-color: #33aca8; background: #33aca8; box-shadow: none; transform: scale(1); }
  95%, 100% { border-color: rgba(255,255,255,0.2); background: var(--lynq-bg); box-shadow: none; transform: scale(1); }
}
@keyframes fw-dot2 {
  0%, 23%   { border-color: rgba(255,255,255,0.2); background: var(--lynq-bg); box-shadow: none; transform: scale(1); }
  24%       { border-color: #4fd1cb; background: #4fd1cb; box-shadow: 0 0 8px 2px rgba(79,209,203,0.7); transform: scale(1.25); }
  28%, 90%  { border-color: #33aca8; background: #33aca8; box-shadow: none; transform: scale(1); }
  95%, 100% { border-color: rgba(255,255,255,0.2); background: var(--lynq-bg); box-shadow: none; transform: scale(1); }
}
@keyframes fw-dot3 {
  0%, 41%   { border-color: rgba(255,255,255,0.2); background: var(--lynq-bg); box-shadow: none; transform: scale(1); }
  42%       { border-color: #4fd1cb; background: #4fd1cb; box-shadow: 0 0 8px 2px rgba(79,209,203,0.7); transform: scale(1.25); }
  46%, 90%  { border-color: #33aca8; background: #33aca8; box-shadow: none; transform: scale(1); }
  95%, 100% { border-color: rgba(255,255,255,0.2); background: var(--lynq-bg); box-shadow: none; transform: scale(1); }
}
@keyframes fw-dot4 {
  0%, 71%   { border-color: rgba(255,255,255,0.2); background: var(--lynq-bg); box-shadow: none; transform: scale(1); }
  72%       { border-color: #4fd1cb; background: #4fd1cb; box-shadow: 0 0 8px 2px rgba(79,209,203,0.7); transform: scale(1.25); }
  76%, 90%  { border-color: #33aca8; background: #33aca8; box-shadow: none; transform: scale(1); }
  95%, 100% { border-color: rgba(255,255,255,0.2); background: var(--lynq-bg); box-shadow: none; transform: scale(1); }
}
/* columns fade / lift in when the playhead reaches them, hold, reset together */
@keyframes fw-col1 {
  0%, 2%    { opacity: 0; transform: translateY(10px); }
  5%, 90%   { opacity: 1; transform: translateY(0); }
  96%, 100% { opacity: 0; transform: translateY(10px); }
}
@keyframes fw-col2 {
  0%, 23%   { opacity: 0; transform: translateY(10px); }
  26%, 90%  { opacity: 1; transform: translateY(0); }
  96%, 100% { opacity: 0; transform: translateY(10px); }
}
@keyframes fw-col3 {
  0%, 41%   { opacity: 0; transform: translateY(10px); }
  44%, 90%  { opacity: 1; transform: translateY(0); }
  96%, 100% { opacity: 0; transform: translateY(10px); }
}
@keyframes fw-col4 {
  0%, 71%   { opacity: 0; transform: translateY(10px); }
  74%, 90%  { opacity: 1; transform: translateY(0); }
  96%, 100% { opacity: 0; transform: translateY(10px); }
}
/* resource dots: amber (pending) → teal (ready), staggered within the Apply phase */
@keyframes fw-rd-app {
  0%, 50%   { background: var(--lynq-amber); box-shadow: none; }
  54%       { background: #33aca8; box-shadow: 0 0 6px 1px rgba(51,172,168,0.7); }
  58%, 90%  { background: #33aca8; box-shadow: none; }
  95%, 100% { background: var(--lynq-amber); box-shadow: none; }
}
@keyframes fw-rd-svc {
  0%, 55%   { background: var(--lynq-amber); box-shadow: none; }
  59%       { background: #33aca8; box-shadow: 0 0 6px 1px rgba(51,172,168,0.7); }
  63%, 90%  { background: #33aca8; box-shadow: none; }
  95%, 100% { background: var(--lynq-amber); box-shadow: none; }
}
@keyframes fw-rd-web {
  0%, 60%   { background: var(--lynq-amber); box-shadow: none; }
  64%       { background: #33aca8; box-shadow: 0 0 6px 1px rgba(51,172,168,0.7); }
  68%, 90%  { background: #33aca8; box-shadow: none; }
  95%, 100% { background: var(--lynq-amber); box-shadow: none; }
}
/* running badge pops in at the Live phase */
@keyframes fw-badge {
  0%, 74%   { opacity: 0; transform: scale(0.85); }
  78%       { opacity: 1; transform: scale(1.12); }
  82%, 92%  { opacity: 1; transform: scale(1); }
  96%, 100% { opacity: 0; transform: scale(0.85); }
}
/* captions: one per phase, with a small blank gap between each so they never
   overlap (they share the same absolute position — a crossfade would garble). */
@keyframes fw-cap1 {
  0%        { opacity: 0; }
  3%, 20%   { opacity: 1; }
  23%, 100% { opacity: 0; }
}
@keyframes fw-cap2 {
  0%, 23%   { opacity: 0; }
  26%, 37%  { opacity: 1; }
  40%, 100% { opacity: 0; }
}
@keyframes fw-cap3 {
  0%, 43%   { opacity: 0; }
  46%, 66%  { opacity: 1; }
  69%, 100% { opacity: 0; }
}
@keyframes fw-cap4 {
  0%, 72%   { opacity: 0; }
  75%, 92%  { opacity: 1; }
  95%, 100% { opacity: 0; }
}

@media (max-width: 720px) {
  .reconcile { padding: 4rem 1.25rem; }
  .flow-cols { flex-wrap: wrap; }
  .col { flex: 1 1 45%; }
}
</style>
