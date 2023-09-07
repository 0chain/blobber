package util

import "time"

type Node struct {
	transactionDataJSON string
	timeStamp           time.Time
	next                *Node
}

type LinkedList struct {
	head   *Node
	tail   *Node
	length int
}

func (ll *LinkedList) Add(data string, timeStamp time.Time) {
	newNode := &Node{transactionDataJSON: data, timeStamp: timeStamp}
	if ll.length == 0 {
		ll.head = newNode
		ll.tail = newNode
	} else {
		ll.tail.next = newNode
		ll.tail = newNode
	}
	ll.length++
	if ll.length > 50 {
		ll.removeFirst()
	}
}

func (ll *LinkedList) GetTransactions() []string {
	transactions := make([]string, 0, ll.length)
	currentNode := ll.head

	for currentNode != nil {
		transactions = append(transactions, currentNode.transactionDataJSON)
		currentNode = currentNode.next
	}

	return transactions
}

func (ll *LinkedList) removeFirst() {
	if ll.length > 0 {
		ll.head = ll.head.next
		ll.length--
		if ll.length == 0 {
			ll.tail = nil
		}
	}
}

var Last50Transactions = LinkedList{}
