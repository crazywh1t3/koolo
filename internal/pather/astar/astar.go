package astar

import (
	"container/heap"
	"math"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/koolo/internal/game"
)

var directions = []data.Position{
	{X: 0, Y: 1}, {X: 1, Y: 0}, {X: 0, Y: -1}, {X: -1, Y: 0}, // Основни посоки
	{X: 1, Y: 1}, {X: -1, Y: 1}, {X: 1, Y: -1}, {X: -1, Y: -1}, // Диагонални посоки
}

type Node struct {
	data.Position
	Cost     int
	Priority int
	Index    int
	Parent   *Node // НОВО: Добавяме връзка към родителя за по-лесно проследяване на пътя
}

func direction(from, to data.Position) (dx, dy int) {
	dx = to.X - from.X
	dy = to.Y - from.Y
	return
}

func CalculatePath(g *game.Grid, start, goal data.Position) ([]data.Position, int, bool) {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)

	// Използваме map за по-ефективно съхранение на вече посетените възли
	nodes := make(map[data.Position]*Node)

	startNode := &Node{Position: start, Cost: 0, Priority: heuristic(start, goal)}
	nodes[start] = startNode
	heap.Push(&pq, startNode)

	for pq.Len() > 0 {
		current := heap.Pop(&pq).(*Node)

		if current.Position == goal {
			var path []data.Position
			p := current
			for p != nil {
				path = append([]data.Position{p.Position}, path...)
				p = p.Parent
			}
			return path, len(path), true
		}

		// Генериране на съседи с подобрена логика
		for _, dir := range directions {
			neighborPos := data.Position{X: current.X + dir.X, Y: current.Y + dir.Y}

			// Проверка дали съседът е в границите на картата
			if neighborPos.X < 0 || neighborPos.X >= g.Width || neighborPos.Y < 0 || neighborPos.Y >= g.Height {
				continue
			}

			// ПРОМЕНЕНО: Проверка за "рязане" на ъгли при диагонално движение
			if dir.X != 0 && dir.Y != 0 { // Ако е диагонал
				// Проверяваме дали съседните плочки по X и Y са проходими
				if g.CollisionGrid[current.Y][current.X+dir.X] > game.CollisionTypeWalkable || g.CollisionGrid[current.Y+dir.Y][current.X] > game.CollisionTypeWalkable {
					continue
				}
			}

			// Изчисляване на цената за движение до този съсед
			newCost := current.Cost + getCost(g, neighborPos)

			// ПРОМЕНЕНО: Добавяне на "наказание" за смяна на посоката, за да се избегне зигзаг
			if current.Parent != nil {
				curDirX, curDirY := direction(current.Parent.Position, current.Position)
				newDirX, newDirY := direction(current.Position, neighborPos)
				if curDirX != newDirX || curDirY != newDirY {
					newCost++ // Малко наказание за завой
				}
			}

			// Ако сме намерили по-добър път до този съсед
			if node, found := nodes[neighborPos]; !found || newCost < node.Cost {
				priority := newCost + heuristic(neighborPos, goal)
				neighborNode := &Node{
					Position: neighborPos,
					Cost:     newCost,
					Priority: priority,
					Parent:   current,
				}
				nodes[neighborPos] = neighborNode
				heap.Push(&pq, neighborNode)
			}
		}
	}

	return nil, 0, false
}

// ПРОМЕНЕНО: getCost вече взима предвид и съседните плочки
func getCost(grid *game.Grid, pos data.Position) int {
	baseCost := 0
	tileType := grid.CollisionGrid[pos.Y][pos.X]

	switch tileType {
	case game.CollisionTypeWalkable:
		baseCost = 1
	case game.CollisionTypeMonster:
		baseCost = 16
	case game.CollisionTypeObject:
		baseCost = 4
	case game.CollisionTypeLowPriority:
		baseCost = 20
	default:
		return math.MaxInt32 // Непроходимо
	}

	// ПРОМЕНЕНО: Добавяне на "наказание" за близост до стени/препятствия
	// Това кара пътеката да стои по-далеч от стените
	for _, d := range directions {
		checkPos := data.Position{X: pos.X + d.X, Y: pos.Y + d.Y}
		if checkPos.X >= 0 && checkPos.X < grid.Width && checkPos.Y >= 0 && checkPos.Y < grid.Height {
			if grid.CollisionGrid[checkPos.Y][checkPos.X] == game.CollisionTypeNonWalkable {
				baseCost += 2 // Добавяме допълнителна цена, ако е до стена
			}
		}
	}

	return baseCost
}

func heuristic(a, b data.Position) int {
	// Манхатън разстояние с лека корекция за диагонали
	dx := math.Abs(float64(a.X - b.X))
	dy := math.Abs(float64(a.Y - b.Y))
	return int(dx + dy)
}