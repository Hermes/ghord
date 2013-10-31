package chord

import ()

//Cluster Struct
//	self 			Node
//	fingerTable		fingerTable
//	kill			chan bool
//	app				Application/Callback/Delegate
//	log				logger
//	logLevel		int
//	stabalizeMin	time.Duration
//	stabalizeMax	time.Duration
//	numSuccessors	int
//	joined			bool
//	credentials		credentials
//	transport		transport

//NewCluster
// Initiate a new cluster with the given node
// initiate the logger with a log level
// set the rest of the settings with the given config
