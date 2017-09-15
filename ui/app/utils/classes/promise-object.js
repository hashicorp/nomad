import Ember from 'ember';

const { ObjectProxy, PromiseProxyMixin } = Ember;

export default ObjectProxy.extend(PromiseProxyMixin);
