import Helper from '@ember/component/helper';

export function formatID([model, relationship]) {
  const id = model.belongsTo(relationship).id();
  return { id, shortId: id.split('-')[0] };
}

export default Helper.helper(formatID);
