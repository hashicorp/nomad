/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

const ProxyTag = <template>
  <span
    class="badge is-light tooltip"
    role="tooltip"
    aria-label="Consul Connect proxy task"
    data-test-proxy-tag
    ...attributes
  >
    Proxy
  </span>
</template>;

export default ProxyTag;
