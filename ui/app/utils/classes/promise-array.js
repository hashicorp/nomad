import ArrayProxy from '@ember/array/proxy';
import PromiseProxyMixin from '@ember/object/promise-proxy-mixin';
import classic from 'ember-classic-decorator';

@classic
export default class PromiseArray extends ArrayProxy.extend(PromiseProxyMixin) {}
