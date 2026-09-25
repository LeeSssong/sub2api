<template>
  <main class="fixture-layout">
    <header class="fixture-heading"><strong>鹈鹕测智</strong><span>本地验收 · 合成样例数据</span></header>
    <slot />
  </main>
</template>
<style>
.fixture-layout { max-width: 1440px; margin: auto; padding: 24px 32px; }
.fixture-heading { display:flex; flex-wrap:wrap; align-items:baseline; gap:12px; margin-bottom:12px; }
.fixture-heading strong { font-size:24px; }
.fixture-heading span { font-size:12px; color:#8896ab; }
@media (max-width:640px) { .fixture-layout { padding:20px 16px; } }
</style>
