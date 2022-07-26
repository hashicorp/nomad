// @ts-check
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { set } from '@ember/object';
import { stringifyObject } from 'nomad-ui/helpers/stringify-object';

export default class PoliciesPolicyController extends Controller {
  modifiedRules = '';
  // get policyString() {
  //   return stringifyObject([this.model.rulesJSON]);
  // }

  // set policyString() {
  // 	console.log('setting policyString',a,b,c);
  // }

  @action updatePolicy(value, codemirror) {
    codemirror.performLint();
    try {
      const hasLintErrors = codemirror?.state.lint.marked?.length > 0;
      if (hasLintErrors || !JSON.parse(value)) {
        throw new Error('Invalid JSON');
      }
      // set(this, 'JSONError', null);
      set(this, 'modifiedRules', JSON.parse(value));
    } catch (error) {
      console.log('o no', error);
      // set(this, 'JSONError', error);
    }
  }
  @action savePolicy() {
    // console.log('saving', this.modifiedRules);
    this.model.rulesJSON = this.modifiedRules;
    this.model.save();
  }
}
