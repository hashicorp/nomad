import Controller from '@ember/controller';

export default class VariablesVariableIndexController extends Controller {
  // Transform the model format (object) into an iterable array
  get keyValues() {
    return Object.entries(this.model.items).map(([key, value]) => ({
      key,
      value,
    }));
  }
}
