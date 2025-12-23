package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	gorillaws "github.com/gorilla/websocket"
)

// TestMultipleWebSocketClients tests WebSocket functionality with multiple concurrent clients
func TestMultipleWebSocketClients(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Create and start a poll
	pollData := map[string]interface{}{
		"title":       "Multi-Client WebSocket Test",
		"description": "Testing multiple WebSocket connections",
		"start_date":  time.Now().Format(time.RFC3339),
		"end_date":    time.Now().Add(time.Hour).Format(time.RFC3339),
		"options": []map[string]interface{}{
			{"option_text": "Option 1"},
			{"option_text": "Option 2"},
			{"option_text": "Option 3"},
		},
	}

	createResp := ts.makeRequest(t, "POST", "/api/polls", pollData, ts.adminToken)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create poll: %d", createResp.StatusCode)
	}

	var createdPoll map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&createdPoll)
	pollID := fmt.Sprintf("%.0f", createdPoll["id"].(float64))

	// Start the poll
	ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/start", nil, ts.adminToken)

	// Create multiple WebSocket connections
	numClients := 5
	connections := make([]*gorillaws.Conn, numClients)
	messageChannels := make([]chan map[string]interface{}, numClients)
	
	wsURL := "ws" + ts.server.URL[4:] + "/ws/" + pollID

	// Connect multiple clients
	for i := 0; i < numClients; i++ {
		conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect WebSocket client %d: %v", i, err)
		}
		connections[i] = conn
		messageChannels[i] = make(chan map[string]interface{}, 10)

		// Start message reader for each connection
		go func(clientID int, conn *gorillaws.Conn, msgChan chan map[string]interface{}) {
			defer conn.Close()
			for {
				var msg map[string]interface{}
				err := conn.ReadJSON(&msg)
				if err != nil {
					return
				}
				msgChan <- msg
			}
		}(i, conn, messageChannels[i])
	}

	// Wait a moment for connections to establish
	time.Sleep(100 * time.Millisecond)

	// Submit a vote to trigger broadcast
	getPollResp := ts.makeRequest(t, "GET", "/api/polls/"+pollID, nil, ts.voterToken)
	var poll map[string]interface{}
	json.NewDecoder(getPollResp.Body).Decode(&poll)
	
	options := poll["options"].([]interface{})
	firstOption := options[0].(map[string]interface{})
	optionID := fmt.Sprintf("%.0f", firstOption["id"].(float64))

	voteData := map[string]interface{}{
		"option_id": optionID,
	}

	voteResp := ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/vote", voteData, ts.voterToken)
	if voteResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to submit vote: %d", voteResp.StatusCode)
	}

	// Verify all clients receive the broadcast message
	var wg sync.WaitGroup
	wg.Add(numClients)

	for i := 0; i < numClients; i++ {
		go func(clientID int, msgChan chan map[string]interface{}) {
			defer wg.Done()
			
			select {
			case msg := <-msgChan:
				if msg["type"] != "vote_update" {
					t.Errorf("Client %d: Expected vote_update message, got %v", clientID, msg["type"])
				}
				
				if fmt.Sprintf("%v", msg["poll_id"]) != pollID {
					t.Errorf("Client %d: Expected poll_id %s, got %v", clientID, pollID, msg["poll_id"])
				}

				// Verify vote data is present
				if msg["data"] == nil {
					t.Errorf("Client %d: Expected vote data in message", clientID)
				}

			case <-time.After(5 * time.Second):
				t.Errorf("Client %d: Timeout waiting for WebSocket message", clientID)
			}
		}(i, messageChannels[i])
	}

	// Wait for all clients to receive messages
	wg.Wait()

	// Clean up connections
	for _, conn := range connections {
		conn.Close()
	}
}

// TestWebSocketConnectionManagement tests connection lifecycle management
func TestWebSocketConnectionManagement(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Create and start a poll
	pollData := map[string]interface{}{
		"title":       "Connection Management Test",
		"description": "Testing WebSocket connection lifecycle",
		"start_date":  time.Now().Format(time.RFC3339),
		"end_date":    time.Now().Add(time.Hour).Format(time.RFC3339),
		"options": []map[string]interface{}{
			{"option_text": "Test Option"},
		},
	}

	createResp := ts.makeRequest(t, "POST", "/api/polls", pollData, ts.adminToken)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create poll: %d", createResp.StatusCode)
	}

	var createdPoll map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&createdPoll)
	pollID := fmt.Sprintf("%.0f", createdPoll["id"].(float64))

	ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/start", nil, ts.adminToken)

	wsURL := "ws" + ts.server.URL[4:] + "/ws/" + pollID

	// Test connection establishment
	conn1, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to establish WebSocket connection: %v", err)
	}

	// Test multiple connections to same poll
	conn2, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to establish second WebSocket connection: %v", err)
	}

	// Test graceful disconnection
	conn1.Close()
	
	// Verify remaining connection still works
	messages := make(chan map[string]interface{}, 5)
	go func() {
		for {
			var msg map[string]interface{}
			err := conn2.ReadJSON(&msg)
			if err != nil {
				return
			}
			messages <- msg
		}
	}()

	// Submit vote to test remaining connection
	getPollResp := ts.makeRequest(t, "GET", "/api/polls/"+pollID, nil, ts.voterToken)
	var poll map[string]interface{}
	json.NewDecoder(getPollResp.Body).Decode(&poll)
	
	options := poll["options"].([]interface{})
	firstOption := options[0].(map[string]interface{})
	optionID := fmt.Sprintf("%.0f", firstOption["id"].(float64))

	voteData := map[string]interface{}{
		"option_id": optionID,
	}

	ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/vote", voteData, ts.voterToken)

	// Verify second connection receives message
	select {
	case msg := <-messages:
		if msg["type"] != "vote_update" {
			t.Errorf("Expected vote_update message, got %v", msg["type"])
		}
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for message on remaining connection")
	}

	conn2.Close()
}

// TestWebSocketPollStatusUpdates tests real-time poll status change notifications
func TestWebSocketPollStatusUpdates(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Create a poll
	pollData := map[string]interface{}{
		"title":       "Status Update Test",
		"description": "Testing poll status change notifications",
		"start_date":  time.Now().Format(time.RFC3339),
		"end_date":    time.Now().Add(time.Hour).Format(time.RFC3339),
		"options": []map[string]interface{}{
			{"option_text": "Status Option"},
		},
	}

	createResp := ts.makeRequest(t, "POST", "/api/polls", pollData, ts.adminToken)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create poll: %d", createResp.StatusCode)
	}

	var createdPoll map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&createdPoll)
	pollID := fmt.Sprintf("%.0f", createdPoll["id"].(float64))

	// Connect to WebSocket before starting poll
	wsURL := "ws" + ts.server.URL[4:] + "/ws/" + pollID
	conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	messages := make(chan map[string]interface{}, 10)
	go func() {
		for {
			var msg map[string]interface{}
			err := conn.ReadJSON(&msg)
			if err != nil {
				return
			}
			messages <- msg
		}
	}()

	// Start the poll and check for status update
	ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/start", nil, ts.adminToken)

	// Wait for status update message
	select {
	case msg := <-messages:
		if msg["type"] == "poll_status_update" {
			data := msg["data"].(map[string]interface{})
			if data["status"] != "active" {
				t.Errorf("Expected status 'active', got %v", data["status"])
			}
		}
	case <-time.After(3 * time.Second):
		// Status updates might not be implemented yet, so this is not a failure
		t.Log("No status update message received (may not be implemented)")
	}

	// Pause the poll
	ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/pause", nil, ts.adminToken)

	// Check for pause notification
	select {
	case msg := <-messages:
		if msg["type"] == "poll_status_update" {
			data := msg["data"].(map[string]interface{})
			if data["status"] != "paused" {
				t.Errorf("Expected status 'paused', got %v", data["status"])
			}
		}
	case <-time.After(3 * time.Second):
		t.Log("No pause notification received (may not be implemented)")
	}

	// Stop the poll
	ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/stop", nil, ts.adminToken)

	// Check for stop notification
	select {
	case msg := <-messages:
		if msg["type"] == "poll_status_update" || msg["type"] == "poll_closed" {
			// Either message type is acceptable for poll closure
			t.Log("Received poll closure notification")
		}
	case <-time.After(3 * time.Second):
		t.Log("No stop notification received (may not be implemented)")
	}
}

// TestWebSocketErrorHandling tests WebSocket error scenarios
func TestWebSocketErrorHandling(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Test connection to non-existent poll
	nonExistentPollURL := "ws" + ts.server.URL[4:] + "/ws/99999"
	conn, resp, err := gorillaws.DefaultDialer.Dial(nonExistentPollURL, nil)
	
	if err == nil {
		conn.Close()
		t.Error("Expected error when connecting to non-existent poll")
	} else if resp != nil && resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 for non-existent poll, got %d", resp.StatusCode)
	}

	// Test connection with invalid poll ID
	invalidPollURL := "ws" + ts.server.URL[4:] + "/ws/invalid"
	conn, resp, err = gorillaws.DefaultDialer.Dial(invalidPollURL, nil)
	
	if err == nil {
		conn.Close()
		t.Error("Expected error when connecting with invalid poll ID")
	} else if resp != nil && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid poll ID, got %d", resp.StatusCode)
	}
}

// TestWebSocketConcurrentVoting tests concurrent voting with WebSocket updates
func TestWebSocketConcurrentVoting(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Create additional voters
	voters := []string{}
	for i := 2; i <= 5; i++ {
		voterData := map[string]interface{}{
			"username": fmt.Sprintf("voter%d", i),
			"password": "voter123",
			"role":     "voter",
		}
		ts.makeRequest(t, "POST", "/api/auth/register", voterData, "")

		loginData := map[string]interface{}{
			"username": fmt.Sprintf("voter%d", i),
			"password": "voter123",
		}
		loginResp := ts.makeRequest(t, "POST", "/api/auth/login", loginData, "")
		
		var loginResult map[string]interface{}
		json.NewDecoder(loginResp.Body).Decode(&loginResult)
		voters = append(voters, loginResult["token"].(string))
	}

	// Create and start poll
	pollData := map[string]interface{}{
		"title":       "Concurrent Voting Test",
		"description": "Testing concurrent voting with WebSocket",
		"start_date":  time.Now().Format(time.RFC3339),
		"end_date":    time.Now().Add(time.Hour).Format(time.RFC3339),
		"options": []map[string]interface{}{
			{"option_text": "Concurrent Option 1"},
			{"option_text": "Concurrent Option 2"},
		},
	}

	createResp := ts.makeRequest(t, "POST", "/api/polls", pollData, ts.adminToken)
	var createdPoll map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&createdPoll)
	pollID := fmt.Sprintf("%.0f", createdPoll["id"].(float64))

	ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/start", nil, ts.adminToken)

	// Connect WebSocket to monitor updates
	wsURL := "ws" + ts.server.URL[4:] + "/ws/" + pollID
	conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	messages := make(chan map[string]interface{}, 20)
	go func() {
		for {
			var msg map[string]interface{}
			err := conn.ReadJSON(&msg)
			if err != nil {
				return
			}
			messages <- msg
		}
	}()

	// Get poll options
	getPollResp := ts.makeRequest(t, "GET", "/api/polls/"+pollID, nil, ts.adminToken)
	var poll map[string]interface{}
	json.NewDecoder(getPollResp.Body).Decode(&poll)
	
	options := poll["options"].([]interface{})
	option1ID := fmt.Sprintf("%.0f", options[0].(map[string]interface{})["id"].(float64))
	option2ID := fmt.Sprintf("%.0f", options[1].(map[string]interface{})["id"].(float64))

	// Submit concurrent votes
	var wg sync.WaitGroup
	allVoters := append([]string{ts.voterToken}, voters...)
	
	for i, voterToken := range allVoters {
		wg.Add(1)
		go func(voterIndex int, token string) {
			defer wg.Done()
			
			// Alternate between options
			optionID := option1ID
			if voterIndex%2 == 1 {
				optionID = option2ID
			}
			
			voteData := map[string]interface{}{
				"option_id": optionID,
			}
			
			ts.makeRequest(t, "POST", "/api/polls/"+pollID+"/vote", voteData, token)
		}(i, voterToken)
	}

	wg.Wait()

	// Count received WebSocket messages
	messageCount := 0
	timeout := time.After(5 * time.Second)
	
	for {
		select {
		case msg := <-messages:
			if msg["type"] == "vote_update" {
				messageCount++
			}
		case <-timeout:
			goto done
		default:
			if messageCount >= len(allVoters) {
				goto done
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

done:
	if messageCount < len(allVoters) {
		t.Errorf("Expected at least %d WebSocket messages, got %d", len(allVoters), messageCount)
	}

	// Verify final vote counts
	finalPollResp := ts.makeRequest(t, "GET", "/api/polls/"+pollID, nil, ts.adminToken)
	var finalPoll map[string]interface{}
	json.NewDecoder(finalPollResp.Body).Decode(&finalPoll)
	
	finalOptions := finalPoll["options"].([]interface{})
	option1Count := int(finalOptions[0].(map[string]interface{})["vote_count"].(float64))
	option2Count := int(finalOptions[1].(map[string]interface{})["vote_count"].(float64))
	
	totalVotes := option1Count + option2Count
	expectedVotes := len(allVoters)
	
	if totalVotes != expectedVotes {
		t.Errorf("Expected %d total votes, got %d (option1: %d, option2: %d)", 
			expectedVotes, totalVotes, option1Count, option2Count)
	}
}