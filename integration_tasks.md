# Integration and Implementation Task List

This document lists the remaining tasks to fully integrate the frontend with the backend and complete the unimplemented features in the backend.

## 1. Backend Handler Completions
The following handlers in `vote-apps/internal/handlers/` need to be updated to call their respective services instead of returning `NOT_IMPLEMENTED`.

### Poll Handler (`poll.go`)
- [ ] **ListPolls**: Call `pollService.ListPolls` with filters and pagination.
- [ ] **CreatePoll**: Call `pollService.CreatePoll`.
- [ ] **GetPoll**: Call `pollService.GetPoll`.
- [ ] **UpdatePoll**: Call `pollService.UpdatePoll`.
- [ ] **DeletePoll**: Call `pollService.DeletePoll`.
- [ ] **StartPoll**: Call `pollService.StartPoll`.
- [ ] **PausePoll**: Call `pollService.PausePoll`.
- [ ] **StopPoll**: Call `pollService.StopPoll`.

### Vote Handler (`vote.go`)
- [ ] **SubmitVote**: Call `voteService.SubmitVote`.
- [ ] **GetVotePage**: Call `pollService.GetPollWithOptions`.
- [ ] **GetUserVotes**: Call `voteService.GetVoteHistory`.

### Admin Handler (`admin.go`)
- [ ] **CreateUser**: Call `userService.CreateUser`.
- [ ] **UpdateUserStatus**: Call `userService.UpdateUserStatus`.
- [ ] **GetDashboardStats**: Implement `GetDashboardStatistics` in `PollService` and call it from this handler.

## 2. Service Implementation Enhancements
- [ ] **PollService**: Implement `GetDashboardStatistics` to provide summary data for the admin dashboard.
- [ ] **WebSocketService**: Ensure `BroadcastPollStatusUpdate` and other real-time notifications are fully functional.

## 3. Frontend Integration
- [ ] **API URL Configuration**: Verify `.env` in frontend points to the correct backend URL.
- [ ] **Data Format Alignment**: Verify that the JSON response structure from the backend matches the TypeScript interfaces in the frontend (especially for statistics and poll lists).
- [ ] **Authentication Flow**: Test login/register/refresh flow between frontend and backend.

## 4. Verification
- [ ] Test the full user journey: Register -> Login -> View Polls -> Vote -> View Results.
- [ ] Test admin journey: Create Poll -> Manage Poll (Start/Pause/Stop) -> View Participation -> Export Results.
- [ ] Verify real-time updates via WebSockets.
