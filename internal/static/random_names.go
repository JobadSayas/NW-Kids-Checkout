package static

import "math/rand"

func RandomFirstName() string {
	return firstNames[rand.Intn(len(firstNames))]
}

func RandomLastName() string {
	return lastNames[rand.Intn(len(lastNames))]
}

var firstNames = []string{
	"Steve",
	"John",
	"Jane",
	"Matthew",
	"Robert",
	"Joseph",
	"David",
	"Richard",
	"Thomas",
	"Charles",
	"William",
	"Emma",
	"Alice",
	"Oliver",
	"Isabella",
	"Sophia",
	"Elizabeth",
	"Mark",
	"Jacob",
	"Mary",
	"Steven",
	"Jonathan",
}

var lastNames = []string{
	"O'Connell",
	"O'Reilly",
	"O'Hara",
	"O'Neill",
	"O'Brien",
	"Espinoza",
	"Davidson",
	"Alfonso",
	"Scott",
	"Smith",
	"Johnson",
	"Williams",
	"Brown",
	"Jones",
	"Miller",
	"Davis",
	"Garcia",
	"Rodriguez",
	"Wilson",
	"Martinez",
	"Anderson",
	"Taylor",
	"Thomas",
	"Hernandez",
	"Moore",
	"Martin",
	"Stephens",
	"Trenton",
}
