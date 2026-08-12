package casbin

import "context"

func (c *CasbinAdapter) addPolicyFacts(_ context.Context, rules ...PolicyRule) error {
	c.holder.mu.Lock()
	defer c.holder.mu.Unlock()

	for _, rule := range rules {
		_, err := c.holder.enforcer.AddPolicy(rule.Sub, rule.Dom, rule.Obj, rule.Act, normalizeScopeKey(rule.Scope))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *CasbinAdapter) removePolicyFacts(_ context.Context, rules ...PolicyRule) error {
	c.holder.mu.Lock()
	defer c.holder.mu.Unlock()

	for _, rule := range rules {
		_, err := c.holder.enforcer.RemovePolicy(rule.Sub, rule.Dom, rule.Obj, rule.Act, normalizeScopeKey(rule.Scope))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *CasbinAdapter) addGroupingFacts(_ context.Context, rules ...GroupingRule) error {
	c.holder.mu.Lock()
	defer c.holder.mu.Unlock()

	for _, rule := range rules {
		_, err := c.holder.enforcer.AddGroupingPolicy(rule.Sub, rule.Role, rule.Dom)
		if err != nil {
			return err
		}
	}
	return nil
}
