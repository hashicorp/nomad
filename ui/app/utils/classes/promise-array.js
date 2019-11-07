import ArrayProxy from '@ember/array/proxy';
import PromiseProxyMixin from '@ember/object/promise-proxy-mixin';

export default ArrayProxy.extend(PromiseProxyMixin);
