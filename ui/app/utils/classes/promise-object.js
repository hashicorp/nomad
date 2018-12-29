import ObjectProxy from '@ember/object/proxy';
import PromiseProxyMixin from '@ember/object/promise-proxy-mixin';

export default ObjectProxy.extend(PromiseProxyMixin);
