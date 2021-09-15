export default ability => hooks => {
  hooks.beforeEach(function() {
    this.ability = this.owner.lookup(`ability:${ability}`);
    this.can = this.owner.lookup('service:can');
  });

  hooks.afterEach(function() {
    delete this.ability;
    delete this.can;
  });
};
