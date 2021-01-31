export default ability => hooks => {
  hooks.beforeEach(function() {
    this.ability = this.owner.lookup(`ability:${ability}`);
  });

  hooks.afterEach(function() {
    delete this.ability;
  });
};
