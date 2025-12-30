<?php
header('Content-Type: application/json');
header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Methods: GET, POST, OPTIONS');

// Set timezone to CST (Central Standard Time)
date_default_timezone_set('America/Chicago');

// Get current time in CST
$todayCST = date('Y-m-d');

// Calculate times for today between 14:50 and 15:10 CST
$timesCST = [
    '14:50:00',
    '14:52:00', 
    '14:55:00',
    '14:58:00',
    '15:00:00',
    '15:02:00',
    '15:05:00',
    '15:07:00',
    '15:09:00',
    '15:10:00'
];

// Hardcoded children data with CST times
$children = [
    [
        'planning_center_id' => 'pc001',
        'location_id' => 101,
        'first_name' => 'Ethan',
        'last_name' => 'Williams',
        'security_code' => 'Y8F1',
        'checked_out_at' => $todayCST . 'T' . $timesCST[0]
    ],
    [
        'planning_center_id' => 'pc002',
        'location_id' => 102,
        'first_name' => 'Olivia',
        'last_name' => 'Johnson',
        'security_code' => 'Y9R2',
        'checked_out_at' => $todayCST . 'T' . $timesCST[1]
    ],
    [
        'planning_center_id' => 'pc003',
        'location_id' => 103,
        'first_name' => 'Noah',
        'last_name' => 'Smith',
        'security_code' => 'A5B3',
        'checked_out_at' => $todayCST . 'T' . $timesCST[2]
    ],
    [
        'planning_center_id' => 'pc004',
        'location_id' => 104,
        'first_name' => 'Ava',
        'last_name' => 'Brown',
        'security_code' => 'C7D4',
        'checked_out_at' => $todayCST . 'T' . $timesCST[3]
    ],
    [
        'planning_center_id' => 'pc005',
        'location_id' => 105,
        'first_name' => 'Liam',
        'last_name' => 'Jones',
        'security_code' => 'E2F5',
        'checked_out_at' => $todayCST . 'T' . $timesCST[4]
    ],
    [
        'planning_center_id' => 'pc006',
        'location_id' => 106,
        'first_name' => 'Emma',
        'last_name' => 'Davis',
        'security_code' => 'G8H6',
        'checked_out_at' => $todayCST . 'T' . $timesCST[5]
    ],
    [
        'planning_center_id' => 'pc007',
        'location_id' => 107,
        'first_name' => 'Mason',
        'last_name' => 'Miller',
        'security_code' => 'I1J7',
        'checked_out_at' => $todayCST . 'T' . $timesCST[6]
    ],
    [
        'planning_center_id' => 'pc008',
        'location_id' => 108,
        'first_name' => 'Sophia',
        'last_name' => 'Wilson',
        'security_code' => 'K3L8',
        'checked_out_at' => $todayCST . 'T' . $timesCST[7]
    ],
    [
        'planning_center_id' => 'pc009',
        'location_id' => 109,
        'first_name' => 'James',
        'last_name' => 'Taylor',
        'security_code' => 'M9N0',
        'checked_out_at' => $todayCST . 'T' . $timesCST[8]
    ],
    [
        'planning_center_id' => 'pc010',
        'location_id' => 110,
        'first_name' => 'Isabella',
        'last_name' => 'Moore',
        'security_code' => 'O4P1',
        'checked_out_at' => $todayCST . 'T' . $timesCST[9]
    ]
];

// Shuffle to simulate different order
shuffle($children);

// Sort by checked_out_at (most recent first)
usort($children, function($a, $b) {
    return strtotime($b['checked_out_at']) - strtotime($a['checked_out_at']);
});

// Add a random element: sometimes return 8-10 children, sometimes all 10
$randomCount = rand(8, 10);
$responseData = array_slice($children, 0, $randomCount);

echo json_encode($responseData, JSON_PRETTY_PRINT);
?>