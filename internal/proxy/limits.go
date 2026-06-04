package proxy

const MaxRules = 64

func WouldExceedLimit(currentEntries int, newRules int) bool {
	return currentEntries+newRules > MaxRules
}

func FindCollisions(current *ReverseProxyJSON, hostnames []string) []string {
	existing := make(map[string]bool)
	for _, entry := range current.List {
		if !entry.QnapDockerMdnsManaged {
			existing[entry.ServerName] = true
		}
	}

	var collisions []string
	for _, h := range hostnames {
		if existing[h] {
			collisions = append(collisions, h)
		}
	}
	return collisions
}

func FilterCollisions(hostnames []string, collisions []string) []string {
	collisionSet := make(map[string]bool)
	for _, c := range collisions {
		collisionSet[c] = true
	}
	var filtered []string
	for _, h := range hostnames {
		if !collisionSet[h] {
			filtered = append(filtered, h)
		}
	}
	return filtered
}
