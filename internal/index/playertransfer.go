package index

const (
	// PlayerTransferByPlayer indexes PlayerTransfers by each player UUID in
	// their spec, so a presence event can find the transfers waiting on that
	// player without scanning every pending transfer.
	PlayerTransferByPlayer = "playertransfer.byPlayer"
)
