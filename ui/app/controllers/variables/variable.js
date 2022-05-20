import Controller from '@ember/controller';
import { tracked } from '@glimmer/tracking';

export default class VariablesVariableController extends Controller {
  get breadcrumb() {
    return {
      label: this.model.path,
      args: [`variables.variable`, this.model.path],
    };
  }

  // Transform the model format (object) into an iterable array
  get keyValues() {
    return Object.entries(this.model.items).map(([key, value]) => ({
      key,
      value,
    }));
  }
}
